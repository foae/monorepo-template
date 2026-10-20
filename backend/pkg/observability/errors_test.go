package observability

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type sqlStateError string

func (e sqlStateError) Error() string    { return "password=credential-canary" }
func (e sqlStateError) SQLState() string { return string(e) }

func TestErrorKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, "none"},
		{"unknown", errors.New("password=credential-canary"), "unknown"},
		{"canceled", context.Canceled, "canceled"},
		{"deadline", context.DeadlineExceeded, "deadline_exceeded"},
		{"refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, "connection_refused"},
		{"dns missing", &net.DNSError{Name: "credential-canary", IsNotFound: true}, "dns_not_found"},
		{"dns timeout", &net.DNSError{Name: "credential-canary", IsTimeout: true}, "dns_timeout"},
		{"dns", &net.DNSError{Name: "credential-canary"}, "dns"},
		{"network timeout", &net.OpError{Op: "dial", Err: context.DeadlineExceeded}, "deadline_exceeded"},
		{"network timeout errno", &net.OpError{Op: "dial", Err: syscall.ETIMEDOUT}, "network_timeout"},
		{"certificate", x509.HostnameError{Host: "credential-canary"}, "tls_certificate"},
		{"tls verification", &tls.CertificateVerificationError{Err: errors.New("credential-canary")}, "tls_certificate"},
		{"tls record", tls.RecordHeaderError{Msg: "credential-canary"}, "tls"},
		{"postgres", sqlStateError("23505"), "postgres_23505"},
		{"postgres letters", sqlStateError("42P01"), "postgres_42P01"},
		{"invalid sqlstate", sqlStateError("secret-canary"), "postgres"},
		{"lowercase sqlstate", sqlStateError("42p01"), "postgres"},
		{"non ascii sqlstate", sqlStateError("ABé1"), "postgres"},
		{"dirty migration", migrate.ErrDirty{Version: 12345}, "migration_dirty"},
		{"grpc unavailable", status.Error(codes.Unavailable, "credential-canary"), "grpc_Unavailable"},
		{"dirty migration pointer", &migrate.ErrDirty{Version: 12345}, "migration_dirty"},
		{"grpc arbitrary code", status.Error(codes.Code(9999), "credential-canary"), "grpc_unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.err
			if err != nil {
				err = fmt.Errorf("credential-canary: %w", err)
			}
			if got := ErrorKind(err); got != tc.want {
				t.Fatalf("kind = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPanicStackOmitsArguments(t *testing.T) {
	t.Parallel()
	stack := stackWithSecret("credential-canary")
	if strings.Contains(stack, "credential-canary") || !strings.Contains(stack, "stackWithSecret") || !strings.Contains(stack, "errors_test.go:") {
		t.Fatalf("unexpected static stack: %s", stack)
	}
}

//go:noinline
func stackWithSecret(secret string) string {
	stack := PanicStack()
	if len(secret) == 0 {
		return ""
	}
	return stack
}
