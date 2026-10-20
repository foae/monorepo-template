package observability

import (
	"context"
	"errors"
	"math"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	collector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type traceCollector struct {
	collector.UnimplementedTraceServiceServer
	mu      sync.Mutex
	spans   int
	block   bool
	entered chan struct{}
}

func (c *traceCollector) Export(ctx context.Context, request *collector.ExportTraceServiceRequest) (*collector.ExportTraceServiceResponse, error) {
	if c.block {
		select {
		case c.entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, resource := range request.ResourceSpans {
		for _, scope := range resource.ScopeSpans {
			c.spans += len(scope.Spans)
		}
	}
	return &collector.ExportTraceServiceResponse{}, nil
}

func startCollector(t *testing.T, collectorServer *traceCollector) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	collector.RegisterTraceServiceServer(server, collectorServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String()
}

// These tests change OTel globals and deliberately do not run in parallel.
func preserveGlobals(t *testing.T) {
	t.Helper()
	provider, propagator, handler := otel.GetTracerProvider(), otel.GetTextMapPropagator(), otel.GetErrorHandler()
	t.Cleanup(func() {
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagator)
		otel.SetErrorHandler(handler)
	})
}

func TestTracingExportsEachSpanOnce(t *testing.T) {
	preserveGlobals(t)
	server := &traceCollector{}
	shutdown, err := SetupTracing(context.Background(), OTelConfig{Endpoint: startCollector(t, server), SampleRatio: 1, Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	for range 3 {
		_, span := otel.Tracer("test").Start(context.Background(), "one")
		span.End()
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.spans != 3 {
		t.Fatalf("exported %d spans, want 3", server.spans)
	}
}

func TestTracingDisabledAndValidation(t *testing.T) {
	preserveGlobals(t)
	provider, propagator := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4317")
	shutdown, err := SetupTracing(context.Background(), OTelConfig{})
	if err != nil || shutdown(context.Background()) != nil {
		t.Fatalf("disabled setup: %v", err)
	}
	if otel.GetTracerProvider() != provider || !reflect.DeepEqual(otel.GetTextMapPropagator(), propagator) {
		t.Fatal("disabled setup changed global tracing")
	}
	for _, tc := range []struct {
		name     string
		endpoint string
		ratio    float64
	}{
		{"url", "https://secret@collector:4317/path", 1},
		{"missing port", "collector", 1},
		{"empty host", ":4317", 1},
		{"invalid port", "collector:65536", 1},
		{"nan", "collector:4317", math.NaN()},
		{"infinite", "collector:4317", math.Inf(1)},
		{"negative", "collector:4317", -0.1},
		{"above one", "collector:4317", 1.1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SetupTracing(context.Background(), OTelConfig{Endpoint: tc.endpoint, SampleRatio: tc.ratio})
			if err == nil {
				t.Fatal("expected invalid configuration error")
			}
		})
	}
}

func TestTracingParentSamplingAndPropagation(t *testing.T) {
	preserveGlobals(t)
	for _, ratio := range []float64{0, 1} {
		shutdown, err := SetupTracing(context.Background(), OTelConfig{Endpoint: startCollector(t, &traceCollector{}), SampleRatio: ratio, Insecure: true})
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name    string
			parent  bool
			sampled bool
			remote  bool
		}{
			{"root", false, ratio == 1, false},
			{"remote sampled", true, true, true},
			{"remote unsampled", true, false, true},
			{"local sampled", true, true, false},
			{"local unsampled", true, false, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()
				if tc.parent {
					flags := trace.TraceFlags(0)
					if tc.sampled {
						flags = trace.FlagsSampled
					}
					parent := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: flags, Remote: tc.remote})
					ctx = trace.ContextWithSpanContext(ctx, parent)
				}
				_, span := otel.Tracer("test").Start(ctx, tc.name)
				defer span.End()
				if span.SpanContext().IsSampled() != tc.sampled {
					t.Fatalf("sampling differs from expected %v", tc.sampled)
				}
			})
		}
		carrier := propagation.MapCarrier{"traceparent": "00-01000000000000000000000000000000-0200000000000000-01", "baggage": "region=eu"}
		ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
		out := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(ctx, out)
		if out.Get("traceparent") != carrier.Get("traceparent") || out.Get("baggage") != "region=eu" {
			t.Fatalf("propagation lost W3C context: %v", out)
		}
		if err := shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTracingShutdownHonorsDeadline(t *testing.T) {
	preserveGlobals(t)
	server := &traceCollector{block: true, entered: make(chan struct{}, 1)}
	shutdown, err := SetupTracing(context.Background(), OTelConfig{Endpoint: startCollector(t, server), SampleRatio: 1, Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	_, span := otel.Tracer("test").Start(context.Background(), "blocked")
	span.End()
	provider := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	flushDone := make(chan error, 1)
	flushCtx, cancelFlush := context.WithCancel(context.Background())
	defer cancelFlush()
	go func() { flushDone <- provider.ForceFlush(flushCtx) }()
	select {
	case <-server.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("export did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown error = %v, want context deadline", err)
	}
	cancelFlush()
	select {
	case <-flushDone:
	case <-time.After(5 * time.Second):
		t.Fatal("flush did not honor cancellation")
	}
}
