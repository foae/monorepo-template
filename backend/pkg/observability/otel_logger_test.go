package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
)

func TestExporterInternalDiagnosticsAreSecretSafeJSON(t *testing.T) {
	// OTel, slog, log and environment state are process-global.
	preserveGlobals(t)
	var logs, plaintext bytes.Buffer
	previous := slog.Default()
	previousOutput := log.Writer()
	slog.SetDefault(NewLogger(&logs, slog.LevelDebug))
	log.SetOutput(&plaintext)
	t.Cleanup(func() {
		otel.SetLogger(logr.Discard())
		slog.SetDefault(previous)
		log.SetOutput(previousOutput)
	})
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization=header-canary,malformed-header-canary")
	t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", "/certificate-canary/missing.pem")
	shutdown, err := SetupTracing(context.Background(), OTelConfig{Endpoint: "endpoint-canary.invalid:4317", Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	otel.Handle(errors.New("async-canary"))
	if plaintext.Len() != 0 {
		t.Fatalf("plain-text diagnostic emitted: %q", plaintext.String())
	}
	for _, secret := range []string{"endpoint-canary", "header-canary", "certificate-canary", "async-canary"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("diagnostic leaked %s", secret)
		}
	}
	internal := false
	for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("non-JSON diagnostic: %q", line)
		}
		if record["operation"] == "otel_internal" && record["error_kind"] != nil {
			internal = true
		}
	}
	if !internal {
		t.Fatal("exporter initialization produced no safe internal diagnostic")
	}
}

func TestInternalLoggerDropsArbitraryFieldsAndRespectsLevel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		level slog.Level
		debug bool
	}{
		{"dev", slog.LevelDebug, true},
		{"prod", slog.LevelInfo, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			logger := logr.New(otelLogSink{logger: NewLogger(&logs, tc.level)}).
				WithName("name-canary").WithValues("key-canary", "value-canary")
			if logger.V(1).Enabled() != tc.debug {
				t.Fatal("internal verbosity does not match configured slog level")
			}
			logger.V(1).Info("message-canary", "field-canary", "payload-canary")
			if (logs.Len() > 0) != tc.debug {
				t.Fatal("debug diagnostic filtering failed")
			}
			logger.Error(context.DeadlineExceeded, "error-message-canary", "error-key-canary", "error-value-canary")
			if strings.Contains(logs.String(), "canary") || !strings.Contains(logs.String(), `"error_kind":"deadline_exceeded"`) {
				t.Fatal("internal error did not retain only bounded classification")
			}
		})
	}
}
