package observability

import (
	"context"
	"log/slog"

	"github.com/go-logr/logr"
)

// otelLogSink deliberately discards SDK messages, names and attributes: exporter
// configuration diagnostics can include endpoints, headers and certificate paths.
type otelLogSink struct {
	logger *slog.Logger
}

func (s otelLogSink) Init(logr.RuntimeInfo) {}

func (s otelLogSink) Enabled(verbosity int) bool {
	return s.logger.Enabled(context.Background(), otelLogLevel(verbosity))
}

func (s otelLogSink) Info(verbosity int, _ string, _ ...any) {
	s.logger.Log(context.Background(), otelLogLevel(verbosity), "tracing: internal diagnostic", "operation", "otel_internal", "error_kind", "none")
}

func (s otelLogSink) Error(err error, _ string, _ ...any) {
	s.logger.Error("tracing: internal operation failed", "operation", "otel_internal", "error_kind", ErrorKind(err))
}

func (s otelLogSink) WithValues(...any) logr.LogSink { return s }
func (s otelLogSink) WithName(string) logr.LogSink   { return s }

func otelLogLevel(verbosity int) slog.Level {
	if verbosity > 0 {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
