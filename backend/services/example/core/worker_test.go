package core

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestWorkerRecoversAndRunsNextTask(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	w := NewWorker(slog.New(slog.NewJSONHandler(&logs, nil)))
	calls := 0
	w.task = func(context.Context) {
		calls++
		if calls == 1 {
			panic("secret-payload")
		}
	}
	w.run(context.Background())
	w.run(context.Background())
	if calls != 2 {
		t.Fatalf("ran %d tasks", calls)
	}
	if !strings.Contains(logs.String(), "task panicked") || strings.Contains(logs.String(), "secret-payload") {
		t.Fatalf("unsafe or missing recovery log: %s", logs.String())
	}
}

func TestWorkerCancellationSkipsStartupTask(t *testing.T) {
	t.Parallel()
	w := NewWorker(slog.Default())
	w.task = func(context.Context) { t.Error("task ran after cancellation") }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	StartWorker(ctx, w, time.Hour)
}
