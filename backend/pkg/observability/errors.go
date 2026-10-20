package observability

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorKind classifies errors without exposing messages, addresses or payloads.
// SQLSTATE is the only externally supplied label, bounded to five ASCII characters.
func ErrorKind(err error) string {
	if err == nil {
		return "none"
	}
	var diagnostic *diagnosticError
	if errors.As(err, &diagnostic) {
		return diagnostic.kind
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection_refused"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		switch {
		case dns.IsTimeout:
			return "dns_timeout"
		case dns.IsNotFound:
			return "dns_not_found"
		default:
			return "dns"
		}
	}
	var verification *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalidCertificate x509.CertificateInvalidError
	var roots x509.SystemRootsError
	if errors.As(err, &verification) || errors.As(err, &unknownAuthority) || errors.As(err, &hostname) || errors.As(err, &invalidCertificate) || errors.As(err, &roots) {
		return "tls_certificate"
	}
	var record tls.RecordHeaderError
	var alert tls.AlertError
	if errors.As(err, &record) || errors.As(err, &alert) {
		return "tls"
	}
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		code := state.SQLState()
		if len(code) == 5 {
			valid := true
			for i := range len(code) {
				if !(code[i] >= 'A' && code[i] <= 'Z' || code[i] >= '0' && code[i] <= '9') {
					valid = false
					break
				}
			}
			if valid {
				return "postgres_" + code
			}
		}
		return "postgres"
	}
	var dirty migrate.ErrDirty
	var dirtyPointer *migrate.ErrDirty
	if errors.As(err, &dirty) || errors.As(err, &dirtyPointer) {
		return "migration_dirty"
	}
	if grpcStatus, ok := status.FromError(err); ok {
		switch code := grpcStatus.Code(); code {
		case codes.OK, codes.Canceled, codes.Unknown, codes.InvalidArgument, codes.DeadlineExceeded,
			codes.NotFound, codes.AlreadyExists, codes.PermissionDenied, codes.ResourceExhausted,
			codes.FailedPrecondition, codes.Aborted, codes.OutOfRange, codes.Unimplemented,
			codes.Internal, codes.Unavailable, codes.DataLoss, codes.Unauthenticated:
			return "grpc_" + code.String()
		default:
			return "grpc_unknown"
		}
	}
	var network net.Error
	if errors.As(err, &network) {
		if network.Timeout() {
			return "network_timeout"
		}
		return "network"
	}
	return "unknown"
}

// diagnosticError retains only a safe classification, never the original payload.
type diagnosticError struct {
	operation string
	kind      string
}

func (e *diagnosticError) Error() string { return e.operation + ": " + e.kind }
