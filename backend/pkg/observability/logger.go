package observability

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// NewLogger returns a JSON logger that correlates context-bearing records with
// their trace and span. Correlation fields follow the active slog group.
func NewLogger(writer io.Writer, level slog.Level) *slog.Logger {
	return slog.New(traceHandler{Handler: slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			// Log route templates, not user-controlled URLs or path parameters.
			if len(groups) > 0 && groups[len(groups)-1] == "request" {
				switch attr.Key {
				case "path", "query", "params", "referer":
					return slog.Attr{}
				}
			}
			return attr
		},
	})})
}

type traceHandler struct {
	slog.Handler
}

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		record = record.Clone()
		record.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{Handler: h.Handler.WithGroup(name)}
}
