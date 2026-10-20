package postgres

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestNewCanceledContext(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		ctx  func() (context.Context, context.CancelFunc)
	}{
		{
			name: "canceled",
			err:  context.Canceled,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
		},
		{
			name: "expired",
			err:  context.DeadlineExceeded,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Unix(0, 0))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := tc.ctx()
			defer cancel()
			client, err := New(ctx, "postgres://user@127.0.0.1:1/database?sslmode=disable", 0, 1)
			if client != nil {
				client.Close()
				t.Fatal("canceled initialization returned a client")
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("initialization error = %v, want %v", err, tc.err)
			}
		})
	}
}

func TestNewCancelsPendingConnect(t *testing.T) {
	t.Parallel()
	// A stalled transport tests connection cancellation without mocking SQL or
	// depending on the availability of a PostgreSQL server.
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		client, err := New(ctx, "postgres://user@"+listener.Addr().String()+"/database?sslmode=disable", 0, 1)
		if client != nil {
			client.Close()
		}
		result <- err
	}()

	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("initialization error = %v, want cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pool initialization did not honor cancellation")
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, conn); err != nil {
		t.Fatalf("pool left its pending connection open: %v", err)
	}
}
