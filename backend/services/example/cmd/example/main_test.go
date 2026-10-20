package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestServeWaitsForWorker(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	var stopping atomic.Bool
	go func() {
		drained, err := serve(ctx, &http.Server{}, listener, &stopping, 5*time.Second, 0, func(ctx context.Context) {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-release
		})
		if !drained && err == nil {
			t.Error("undrained service reported success")
		}
		result <- err
	}()
	<-started
	cancel()
	<-canceled
	select {
	case err := <-result:
		t.Fatalf("returned before worker completed: %v", err)
	default:
	}
	if !stopping.Load() {
		t.Error("readiness not disabled")
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestServeReportsUnexpectedServerExit(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
	var stopping atomic.Bool
	drained, err := serve(context.Background(), &http.Server{}, listener, &stopping, time.Second, 0, func(ctx context.Context) { <-ctx.Done() })
	if !drained || err == nil {
		t.Fatalf("drained=%v error=%v", drained, err)
	}
}

func TestServeWorkerDeadline(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	release := make(chan struct{})
	done := make(chan struct{})
	defer func() { close(release); <-done }()
	var stopping atomic.Bool
	drained, err := serve(ctx, &http.Server{}, listener, &stopping, time.Nanosecond, 0, func(context.Context) { defer close(done); <-release })
	if drained || err == nil {
		t.Fatalf("drained=%v error=%v", drained, err)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		key, value string
	}{
		{"ENV_MODE", "invalid"},
		{"POSTGRES_URL", ""},
		{"WORKER_INTERVAL", "0s"},
		{"WORKER_INTERVAL", "credential-canary"},
		{"GRACEFUL_SHUTDOWN_TIMEOUT", "-1s"},
		{"HTTP_DRAIN_DELAY", "-1s"},
		{"HTTP_DRAIN_DELAY", "90s"},
		{"POSTGRES_URL_HOST", "user:credential-canary@host"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			t.Setenv("POSTGRES_URL", "postgres://user:credential-canary@localhost/example")
			t.Setenv("POSTGRES_URL_HOST", "")
			t.Setenv("ENV_MODE", "prod")
			t.Setenv("WORKER_INTERVAL", "1h")
			t.Setenv("GRACEFUL_SHUTDOWN_TIMEOUT", "90s")
			t.Setenv("HTTP_DRAIN_DELAY", "5s")
			t.Setenv(tc.key, tc.value)
			_, err := loadConfig()
			if err == nil || strings.Contains(err.Error(), "credential-canary") {
				t.Fatalf("invalid configuration was accepted or leaked a value: %v", err)
			}
		})
	}
}

func TestServeDrainsOrCancelsHTTPRequest(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "drain"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan struct{})
			release := make(chan struct{})
			handlerDone := make(chan struct{})
			workerStopped := make(chan struct{})
			srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(handlerDone)
				close(entered)
				select {
				case <-release:
					_, _ = io.WriteString(w, "completed")
				case <-r.Context().Done():
				}
			})}
			timeout := 5 * time.Second
			if deadline {
				timeout = time.Nanosecond
			}
			var stopping atomic.Bool
			type outcome struct {
				drained bool
				err     error
			}
			result := make(chan outcome, 1)
			go func() {
				drained, err := serve(ctx, srv, listener, &stopping, timeout, 0, func(ctx context.Context) {
					<-ctx.Done()
					close(workerStopped)
				})
				result <- outcome{drained, err}
			}()
			clientDone := make(chan struct{})
			go func() {
				defer close(clientDone)
				response, err := http.Get("http://" + listener.Addr().String())
				if err != nil {
					if !deadline {
						t.Error(err)
					}
					return
				}
				defer response.Body.Close()
				body, err := io.ReadAll(response.Body)
				if !deadline && (err != nil || string(body) != "completed") {
					t.Errorf("request did not drain: %q %v", body, err)
				}
			}()
			<-entered
			cancel()
			<-workerStopped
			if !deadline {
				select {
				case got := <-result:
					t.Fatalf("returned with active HTTP request: %+v", got)
				default:
				}
				close(release)
			}
			got := <-result
			<-handlerDone
			<-clientDone
			if got.drained == deadline || (got.err != nil) != deadline || !stopping.Load() {
				t.Fatalf("unexpected shutdown state: %+v ready=%v", got, !stopping.Load())
			}
		})
	}
}
