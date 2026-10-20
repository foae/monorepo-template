package httputil

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRecoverer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		status  int
		body    string
	}{
		{"before write", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "999")
			w.Header().Set("X-Secret", "secret")
			panic("secret")
		}, 500, "{\"error\":\"internal_server_error\",\"message\":\"internal server error\"}\n"},
		{"after body", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "partial"); panic("secret") }, 200, "partial"},
		{"after header", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(202); panic("secret") }, 202, ""},
		{"after flush", func(w http.ResponseWriter, _ *http.Request) { w.(http.Flusher).Flush(); panic("secret") }, 200, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			var aborted any
			func() {
				defer func() { aborted = recover() }()
				Recoverer(tc.handler).ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
			}()
			if (tc.status == 500 && aborted != nil) || (tc.status != 500 && aborted != http.ErrAbortHandler) {
				t.Fatalf("unexpected recovery outcome: %v", aborted)
			}
			if recorder.Code != tc.status || recorder.Body.String() != tc.body {
				t.Fatalf("got %d %q, want %d %q", recorder.Code, recorder.Body.String(), tc.status, tc.body)
			}
			if tc.status == 500 && (recorder.Header().Get("Content-Type") != "application/json" || recorder.Header().Get("Content-Length") != "" || recorder.Header().Get("X-Secret") != "") {
				t.Fatalf("unsafe recovery headers: %v", recorder.Header())
			}
		})
	}
}

func TestRecovererSafeJSONLog(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("credential-secret") })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?token=credential-secret", nil))
	if strings.Contains(logs.String(), "credential-secret") || !strings.Contains(logs.String(), `"msg":"HTTP handler panicked"`) {
		t.Fatalf("unsafe or missing panic log: %s", logs.String())
	}
}

func TestRecovererPreservesAbort(t *testing.T) {
	t.Parallel()
	defer func() {
		if got := recover(); got != http.ErrAbortHandler {
			t.Fatalf("panic = %v, want ErrAbortHandler", got)
		}
	}()
	Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

func TestRecovererObservedStatus(t *testing.T) {
	t.Parallel()
	service := "recoverer_status_test"
	reqCounterVec.DeleteLabelValues("500", "GET", service)
	t.Cleanup(func() { reqCounterVec.DeleteLabelValues("500", "GET", service) })
	observed := 0
	requestLogger := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			observed = ww.Status()
		})
	}
	h := requestLogger(ReqMonitor(service)(Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret") }))))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if observed != 500 {
		t.Fatalf("request logger observed %d", observed)
	}
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "http_requests_total" {
			continue
		}
		for _, metric := range family.Metric {
			labels := map[string]string{}
			for _, label := range metric.Label {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["service"] == service && labels["code"] == "500" && metric.GetCounter().GetValue() == 1 {
				return
			}
		}
	}
	t.Fatal("metrics did not observe recovered 500")
}

type hijackResponse struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (w *hijackResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

func (w *hijackResponse) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(w.ResponseRecorder, r)
}

func TestRecovererPreservesHijackInterfaces(t *testing.T) {
	t.Parallel()
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	w := &hijackResponse{httptest.NewRecorder(), server}
	defer func() {
		if got := recover(); got != http.ErrAbortHandler {
			t.Fatalf("panic = %v, want ErrAbortHandler", got)
		}
		if w.Body.Len() != 0 || len(w.Header()) != 0 {
			t.Fatal("recovery wrote after hijack")
		}
		if _, err := server.Write([]byte("closed")); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("hijacked connection remained open: %v", err)
		}
	}()
	Recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("lost Flusher")
		}
		if _, ok := w.(io.ReaderFrom); !ok {
			t.Fatal("lost ReaderFrom")
		}
		if _, _, err := w.(http.Hijacker).Hijack(); err != nil {
			t.Fatal(err)
		}
		panic("secret")
	})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
}

type pushResponse struct {
	*httptest.ResponseRecorder
	target string
}

func (w *pushResponse) Push(target string, _ *http.PushOptions) error {
	w.target = target
	return nil
}

func TestRecovererPreservesHTTP2Push(t *testing.T) {
	t.Parallel()
	w := &pushResponse{ResponseRecorder: httptest.NewRecorder()}
	r := httptest.NewRequest("GET", "/", nil)
	r.ProtoMajor = 2
	Recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Hijacker); ok {
			t.Fatal("advertised unsupported Hijacker")
		}
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("lost Flusher")
		}
		if err := w.(http.Pusher).Push("/asset", nil); err != nil {
			t.Fatal(err)
		}
	})).ServeHTTP(w, r)
	if w.target != "/asset" {
		t.Fatal("Push did not reach underlying writer")
	}
}

func TestRecovererAbortsPartialResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_, _ = io.WriteString(w, "alive")
			return
		}
		_, _ = io.WriteString(w, "partial")
		w.(http.Flusher).Flush()
		panic("secret")
	})))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "partial" || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("partial response falsely completed: %q, %v", body, err)
	}
	response, err = server.Client().Get(server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "alive" {
		t.Fatalf("service did not survive panic: %d %q %v", response.StatusCode, body, err)
	}
}
