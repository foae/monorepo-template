package core

import (
	"context"
	"log/slog"
	"time"

	"monorepo-template/backend/pkg/observability"
)

// Worker runs periodic background tasks.
type Worker struct {
	logger *slog.Logger
	task   func(context.Context)
}

// NewWorker creates a new background worker.
func NewWorker(logger *slog.Logger) *Worker {
	return &Worker{
		logger: logger,
		task: func(ctx context.Context) {
			logger.DebugContext(ctx, "worker: heartbeat")
		},
	}
}

// StartWorker runs the worker loop. It executes once after a 1-minute startup
// delay, then on a fixed interval. Blocks until ctx is cancelled.
func StartWorker(ctx context.Context, w *Worker, interval time.Duration) {
	if interval <= 0 {
		w.logger.ErrorContext(ctx, "worker: interval must be positive")
		return
	}
	w.logger.InfoContext(ctx, "worker: starting background worker",
		"interval", interval, "startup_delay", "1m")

	// Run once after startup delay
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Minute):
		w.run(ctx)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "worker: shutting down")
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			w.run(ctx)
		}
	}
}

func (w *Worker) run(ctx context.Context) {
	defer func() {
		if recover() != nil {
			// Panic payloads may contain credentials or request data.
			w.logger.ErrorContext(ctx, "worker: task panicked", "stack", observability.PanicStack())
		}
	}()
	w.task(ctx)
}
