package httputil

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"

	"monorepo-template/backend/pkg/observability"
)

// Recoverer contains handler panics without exposing panic values. Install it
// inside request logging and ReqMonitor so those middleware observe recovered 500s.
// http.ErrAbortHandler retains net/http's connection-abort behavior.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		var hijacked net.Conn
		response := recoveryWriter(ww, &hijacked)
		defer func() {
			if value := recover(); value != nil {
				if value == http.ErrAbortHandler {
					panic(value)
				}
				// Log code locations only, never panic values or argument dumps.
				committed := ww.Status() != 0 || hijacked != nil
				status := ww.Status()
				if !committed {
					status = http.StatusInternalServerError
				}
				slog.ErrorContext(r.Context(), "HTTP handler panicked", "status", status, "response_committed", committed, "stack", observability.PanicStack())
				if committed {
					if hijacked != nil {
						_ = hijacked.Close()
					}
					panic(http.ErrAbortHandler)
				}
				// Do not retain response metadata prepared for an aborted success.
				for name := range ww.Header() {
					delete(ww.Header(), name)
				}
				ww.Header().Set("Content-Type", "application/json")
				ww.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(ww, "{\"error\":\"internal_server_error\",\"message\":\"internal server error\"}\n")
			}
		}()
		next.ServeHTTP(response, r)
	})
}

// Track successful hijacks as committed without advertising unsupported optional
// interfaces. Chi preserves the usual HTTP/1 and HTTP/2 writer capabilities.
func recoveryWriter(w chimw.WrapResponseWriter, hijacked *net.Conn) http.ResponseWriter {
	h, ok := w.(http.Hijacker)
	if !ok {
		return w
	}
	tracked := &recoveryHijacker{WrapResponseWriter: w, hijacker: h, hijacked: hijacked}
	flusher, flush := w.(http.Flusher)
	reader, read := w.(io.ReaderFrom)
	switch {
	case flush && read:
		return struct {
			*recoveryHijacker
			http.Flusher
			io.ReaderFrom
		}{tracked, flusher, reader}
	case flush:
		return struct {
			*recoveryHijacker
			http.Flusher
		}{tracked, flusher}
	case read:
		return struct {
			*recoveryHijacker
			io.ReaderFrom
		}{tracked, reader}
	default:
		return tracked
	}
}

type recoveryHijacker struct {
	chimw.WrapResponseWriter
	hijacker http.Hijacker
	hijacked *net.Conn
}

func (w *recoveryHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := w.hijacker.Hijack()
	if err == nil {
		*w.hijacked = conn
	}
	return conn, rw, err
}
