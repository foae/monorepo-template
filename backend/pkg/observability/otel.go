package observability

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
)

const tracingShutdownTimeout = 5 * time.Second

// OTelConfig holds configuration for OpenTelemetry tracing setup.
type OTelConfig struct {
	Env            string
	ServiceName    string
	ServiceVersion string
	Endpoint       string
	Region         string
	SampleRatio    float64
	Insecure       bool
}

// SetupTracing installs tracing only when Endpoint is a host:port address.
// Empty Endpoint leaves the global provider and propagator unchanged.
// The returned function flushes and shuts down the provider within five seconds,
// or the caller's earlier deadline. The provider owns the exporter lifecycle.
func SetupTracing(ctx context.Context, cfg OTelConfig) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if cfg.Endpoint == "" {
		return noop, nil
	}
	if math.IsNaN(cfg.SampleRatio) || math.IsInf(cfg.SampleRatio, 0) || cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return noop, errors.New("tracing: sample ratio must be finite and between 0 and 1")
	}
	host, port, err := net.SplitHostPort(cfg.Endpoint)
	n, portErr := strconv.Atoi(port)
	if err != nil || host == "" || strings.ContainsAny(host, "/@?# \\ \t\r\n") || portErr != nil || n < 1 || n > 65535 {
		return noop, errors.New("tracing: endpoint must be a host:port address")
	}

	// Install before construction: exporter options can log sensitive config.
	otel.SetLogger(logr.New(otelLogSink{logger: slog.Default()}))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Error("tracing: asynchronous operation failed", "operation", "otel_async", "error_kind", ErrorKind(err))
	}))

	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithCompressor(gzip.Name),
		otlptracegrpc.WithTimeout(tracingShutdownTimeout),
	}
	if cfg.Insecure {
		options = append(options, otlptracegrpc.WithInsecure())
	} else {
		options = append(options, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	}
	exp, err := otlptracegrpc.New(ctx, options...)
	if err != nil {
		return noop, &diagnosticError{operation: "tracing: create exporter", kind: ErrorKind(err)}
	}
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironment(cfg.Env),
		attribute.String("service.region", cfg.Region),
	)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxQueueSize(256),
			sdktrace.WithMaxExportBatchSize(128),
			sdktrace.WithExportTimeout(tracingShutdownTimeout)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, tracingShutdownTimeout)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return fmt.Errorf("tracing: shutdown: %w", context.Canceled)
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("tracing: shutdown: %w", context.DeadlineExceeded)
			}
			return &diagnosticError{operation: "tracing: shutdown", kind: ErrorKind(err)}
		}
		return nil
	}, nil
}
