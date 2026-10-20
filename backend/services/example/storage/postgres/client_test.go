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
		name       string
		migrations bool
	}{
		{name: "without migrations"},
		{name: "with migrations", migrations: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			client, err := New(ctx, "postgres://user@127.0.0.1:1/database?sslmode=disable", 0, 1, tc.migrations)
			if client != nil {
				client.Close()
				t.Fatal("canceled initialization returned a client")
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("initialization error = %v, want cancellation", err)
			}
		})
	}
}

func TestRunMigrationsCancelsPendingConnect(t *testing.T) {
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
		result <- runMigrations(ctx, "postgres://user@"+listener.Addr().String()+"/database?sslmode=disable")
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
			t.Fatalf("migration error = %v, want cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("migration connection acquisition did not honor cancellation")
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, conn); err != nil {
		t.Fatalf("migrations left their pending connection open: %v", err)
	}
}
