package core

import (
	"context"
	"log/slog"
	"time"
)

// Worker runs periodic background tasks.
type Worker struct {
	logger *slog.Logger
}

// NewWorker creates a new background worker.
func NewWorker(logger *slog.Logger) *Worker {
	return &Worker{
		logger: logger,
	}
}

// StartWorker runs the worker loop. It executes once after a 1-minute startup
// delay, then on a fixed interval. Blocks until ctx is cancelled.
func StartWorker(ctx context.Context, w *Worker, interval time.Duration) {
	w.logger.Info("worker: starting background worker",
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
			w.logger.Info("worker: shutting down")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

func (w *Worker) run(ctx context.Context) {
	start := time.Now()

	// Replace with your periodic task:
	// - Reconcile metrics with an external source
	// - Clean up expired records
	// - Sync state with an external system
	_ = ctx

	w.logger.Info("worker: periodic tick completed",
		"duration", time.Since(start).Round(time.Millisecond))
}
