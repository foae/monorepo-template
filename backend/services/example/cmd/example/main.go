package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	slogchi "github.com/samber/slog-chi"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"monorepo-template/backend/pkg/envutil"
	"monorepo-template/backend/pkg/httputil"
	"monorepo-template/backend/pkg/observability"
	"monorepo-template/backend/services/example/core"
	"monorepo-template/backend/services/example/handler"
	"monorepo-template/backend/services/example/storage/postgres"

	_ "go.uber.org/automaxprocs"
)

const (
	envDev  = "dev"
	envProd = "prod"
)

type config struct {
	ServiceName     string        `env:"SERVICE_NAME" envDefault:"example"`
	ServiceVersion  string        `env:"SERVICE_VERSION" envDefault:"v1.0.0"`
	EnvMode         string        `env:"ENV_MODE" envDefault:"prod"`
	HTTPListenAddr  string        `env:"HTTP_LISTEN_ADDR" envDefault:"0.0.0.0:8080"`
	OtelEndpoint    string        `env:"OTEL_TRACING_ENDPOINT"`
	PostgresURL     string        `env:"POSTGRES_URL,required"`
	OtelSampleRatio float64       `env:"OTEL_TRACING_SAMPLE_RATIO" envDefault:"0.1"`
	OtelInsecure    bool          `env:"OTEL_TRACING_INSECURE" envDefault:"false"`
	ServiceRegion   string        `env:"SERVICE_REGION" envDefault:"local"`
	WorkerInterval  time.Duration `env:"WORKER_INTERVAL" envDefault:"1h"`
	ShutdownTimeout time.Duration `env:"GRACEFUL_SHUTDOWN_TIMEOUT" envDefault:"90s"`
	DrainDelay      time.Duration `env:"HTTP_DRAIN_DELAY" envDefault:"5s"`
}

// buildLoggerMiddleware logs requests without recording credentials or bodies.
func buildLoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	filters := []slogchi.Filter{
		slogchi.IgnorePath(
			"/health",
			"/healthz",
			"/ready",
			"/metrics",
		),
	}

	cfg := slogchi.Config{
		DefaultLevel:     slog.LevelInfo,
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,
		WithRequestID:    true,
		Filters:          filters,
	}

	return slogchi.NewWithConfig(logger, cfg)
}

func main() {
	// The automaxprocs blank import runs before main; its upstream init log is
	// the only plain-text logging exception.
	slog.SetDefault(observability.NewLogger(os.Stdout, slog.LevelInfo))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		stop() // Restore default signal handling so a second signal forces exit.
	}()
	if err := run(ctx); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func loadConfig() (config, error) {
	var cfg config
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, errors.New("load .env: invalid or unreadable configuration")
	}
	if err := env.Parse(&cfg); err != nil {
		// env errors can contain the supplied value, including credentials.
		var parse env.ParseError
		if errors.As(err, &parse) {
			return cfg, fmt.Errorf("parse configuration field %s: invalid value", parse.Name)
		}
		var missing env.VarIsNotSetError
		if errors.As(err, &missing) {
			return cfg, fmt.Errorf("parse configuration: %s is required", missing.Key)
		}
		return cfg, errors.New("parse configuration: invalid variables or value types")
	}
	if cfg.EnvMode != envDev && cfg.EnvMode != envProd {
		return cfg, errors.New("ENV_MODE must be dev or prod")
	}
	if cfg.PostgresURL == "" {
		return cfg, errors.New("POSTGRES_URL must not be empty")
	}
	if cfg.WorkerInterval <= 0 || cfg.ShutdownTimeout <= 0 || cfg.DrainDelay < 0 {
		return cfg, errors.New("worker and shutdown durations must be positive; drain delay must be nonnegative")
	}
	if cfg.DrainDelay >= cfg.ShutdownTimeout {
		return cfg, errors.New("HTTP_DRAIN_DELAY must be less than GRACEFUL_SHUTDOWN_TIMEOUT")
	}
	var err error
	cfg.PostgresURL, err = envutil.ReplaceHost("POSTGRES_URL", cfg.PostgresURL)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func run(ctx context.Context) (result error) {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	level := slog.LevelInfo
	if cfg.EnvMode == envDev {
		level = slog.LevelDebug
	}
	logger := observability.NewLogger(os.Stdout, level)
	slog.SetDefault(logger)
	slog.InfoContext(ctx, "starting service", "service", cfg.ServiceName, "version", cfg.ServiceVersion)
	slog.DebugContext(ctx, "development logging enabled")

	// Bind synchronously: no false readiness and no wait for a signal on failure.
	listener, err := net.Listen("tcp", cfg.HTTPListenAddr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %s", observability.ErrorKind(err))
	}
	defer listener.Close()

	otelShutdown, err := observability.SetupTracing(ctx, observability.OTelConfig{
		Env: cfg.EnvMode, ServiceName: cfg.ServiceName, ServiceVersion: cfg.ServiceVersion,
		Endpoint: cfg.OtelEndpoint, Region: cfg.ServiceRegion,
		SampleRatio: cfg.OtelSampleRatio, Insecure: cfg.OtelInsecure,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := otelShutdown(context.Background()); err != nil {
			result = errors.Join(result, err)
		}
	}()

	startupCtx, cancelStartup := context.WithTimeout(ctx, 30*time.Second)
	pg, err := postgres.New(startupCtx, cfg.PostgresURL, 1, 20, true)
	cancelStartup()
	if err != nil {
		return fmt.Errorf("initialize database: %s", observability.ErrorKind(err))
	}
	svc, err := core.New(pg)
	if err != nil {
		pg.Close()
		return errors.New("initialize service: database unavailable")
	}

	var shuttingDown atomic.Bool
	h := handler.New(svc, cfg.EnvMode)
	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		buildLoggerMiddleware(logger),
		httputil.ReqMonitor(cfg.ServiceName),
		httputil.Recoverer,
	)
	// Forwarded client IP headers are not trusted without a configured proxy.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "OK")
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if shuttingDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "SHUTTING_DOWN")
			return
		}
		_, _ = io.WriteString(w, "READY")
	})
	r.Handle("/metrics", promhttp.Handler())
	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/items", h.HandleCreateItem())
		api.Get("/items", h.HandleListItems())
		api.Get("/items/{id}", h.HandleGetItem())
		api.Delete("/items/{id}", h.HandleDeleteItem())
	})
	// A disabled tracer provider makes this wrapper a no-op for span export.
	httpSrv := &http.Server{
		Handler: otelhttp.NewHandler(r, cfg.ServiceName, otelhttp.WithFilter(func(r *http.Request) bool {
			switch r.URL.Path {
			case "/health", "/healthz", "/ready", "/metrics":
				return false
			default:
				return true
			}
		})),
		ErrorLog:    log.New(serverErrorWriter{}, "", 0),
		ReadTimeout: 30 * time.Second, ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	worker := core.NewWorker(logger)
	drained, err := serve(ctx, httpSrv, listener, &shuttingDown, cfg.ShutdownTimeout, cfg.DrainDelay,
		func(ctx context.Context) { core.StartWorker(ctx, worker, cfg.WorkerInterval) })
	if drained {
		// Never close dependencies under work that exceeded the shutdown deadline.
		svc.Close()
	}
	return err
}

// net/http diagnostic text can contain panic values, URLs, and peer input.
// Preserve severity without forwarding its unstructured payload.
type serverErrorWriter struct{}

func (serverErrorWriter) Write(p []byte) (int, error) {
	slog.Error("HTTP server transport error")
	return len(p), nil
}

// serve owns goroutines and the shutdown deadline. Dependencies remain alive
// until both requests and the worker finish; main exits nonzero on timeout.
func serve(ctx context.Context, srv *http.Server, listener net.Listener, shuttingDown *atomic.Bool,
	timeout, drainDelay time.Duration, worker func(context.Context),
) (bool, error) {
	requestCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	srv.BaseContext = func(net.Listener) context.Context { return requestCtx }
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		worker(workerCtx)
	}()
	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Serve(listener) }()
	slog.InfoContext(ctx, "service listening", "address", listener.Addr().String(), "go", runtime.Version())

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-serverDone:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = errors.New("HTTP server failed")
		} else {
			serveErr = errors.New("HTTP server stopped unexpectedly")
		}
	}
	shuttingDown.Store(true)
	slog.Info("service shutdown started")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), timeout)
	defer cancelShutdown()
	cancelWorker()
	if serveErr == nil && drainDelay > 0 {
		timer := time.NewTimer(drainDelay)
		select {
		case <-timer.C:
		case <-shutdownCtx.Done():
		}
		timer.Stop()
	}
	httpErr := srv.Shutdown(shutdownCtx)
	if httpErr != nil {
		cancelRequests()
		_ = srv.Close()
	}
	select {
	case <-workerDone:
	case <-shutdownCtx.Done():
		return false, errors.Join(serveErr, errors.New("shutdown deadline exceeded waiting for worker"))
	}
	if httpErr != nil {
		return false, errors.Join(serveErr, errors.New("shutdown deadline exceeded draining HTTP"))
	}
	slog.Info("service shutdown completed")
	return true, serveErr
}
