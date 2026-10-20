package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"monorepo-template/backend/services/example/core"
)

func TestWriteItemErrorDoesNotLeakDetails(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"internal", errors.New("database password=credential-secret"), 500, "internal server error"},
		{"not found", fmt.Errorf("credential-secret: %w", core.ErrItemNotFound), 404, core.ErrItemNotFound.Error()},
		{"access denied", fmt.Errorf("credential-secret: %w", core.ErrItemAccessDenied), 403, core.ErrItemAccessDenied.Error()},
		{"invalid input", fmt.Errorf("%w: credential-secret", core.ErrInvalidInput), 400, core.ErrInvalidInput.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			w := httptest.NewRecorder()
			writeItemError(context.Background(), w, "create_item", tc.err)
			var response ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || response.Error != "create_item_failed" || response.Message != tc.message {
				t.Fatalf("got %d %+v", w.Code, response)
			}
			if strings.Contains(w.Body.String(), "credential-secret") || strings.Contains(logs.String(), "credential-secret") {
				t.Fatal("error details leaked to response or logs")
			}
			if tc.status < 500 && logs.Len() != 0 {
				t.Fatalf("expected HTTP error logged redundantly: %s", logs.String())
			}
			if tc.status == 500 {
				var record map[string]any
				if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
					t.Fatal(err)
				}
				if record["level"] != "ERROR" || record["operation"] != "create_item" || record["status"] != float64(500) || record["error_kind"] != "unknown" {
					t.Fatalf("missing safe failure classification: %v", record)
				}
			}
		})
	}
}

func TestCreateItemRejectsUnsafeBodies(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"multiple documents", `{"name":"first"}{"name":"second"}`, 400},
		{"trailing garbage", `{"name":"first"}garbage`, 400},
		{"malformed", `{"name":`, 400},
		{"oversized field", `{"name":"` + strings.Repeat("a", maxCreateItemBodyBytes) + `"}`, 413},
		{"oversized trailing whitespace", `{"name":"first"}` + strings.Repeat(" ", maxCreateItemBodyBytes), 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest("POST", "/api/v1/items", strings.NewReader(tc.body))
			r.Header.Set("X-Owner-Id", "owner")
			// A nil service ensures invalid input never reaches the core boundary.
			h := New(nil, "test")
			w := httptest.NewRecorder()
			h.HandleCreateItem().ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d", w.Code, tc.status)
			}
		})
	}
}
