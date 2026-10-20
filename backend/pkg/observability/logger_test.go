package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestLoggerContextCorrelation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		group bool
		valid bool
	}{
		{"plain", false, true},
		{"with group", true, true},
		{"without span", false, false},
		{"group without span", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buffer bytes.Buffer
			logger := NewLogger(&buffer, slog.LevelInfo).With("service", "test")
			if tc.group {
				logger = logger.WithGroup("request").With("method", "GET")
			}
			ctx := context.Background()
			sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}})
			if tc.valid {
				ctx = trace.ContextWithSpanContext(ctx, sc)
			}
			logger.DebugContext(ctx, "filtered")
			if buffer.Len() != 0 {
				t.Fatal("debug record bypassed configured level")
			}
			logger.InfoContext(ctx, "hello", "key", "value")
			var record map[string]any
			if err := json.Unmarshal(buffer.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if record["service"] != "test" || record["msg"] != "hello" {
				t.Fatalf("lost attrs: %v", record)
			}
			fields := record
			if tc.group {
				fields = record["request"].(map[string]any)
				if fields["method"] != "GET" {
					t.Fatal("lost grouped attribute")
				}
			}
			if fields["key"] != "value" {
				t.Fatal("lost record attribute")
			}
			if tc.valid {
				if fields["trace_id"] != sc.TraceID().String() || fields["span_id"] != sc.SpanID().String() {
					t.Fatalf("lost correlation: %v", fields)
				}
			} else if _, ok := fields["trace_id"]; ok {
				t.Fatal("invalid context added correlation")
			}
		})
	}
}
