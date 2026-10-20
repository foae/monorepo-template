package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"monorepo-template/backend/pkg/observability"
	"monorepo-template/backend/services/example/core"
)

// mapItemError maps core-layer errors to HTTP status codes.
func mapItemError(err error) int {
	switch {
	case errors.Is(err, core.ErrItemNotFound):
		return http.StatusNotFound
	case errors.Is(err, core.ErrItemAccessDenied):
		return http.StatusForbidden
	case errors.Is(err, core.ErrInvalidInput):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// itemErrorMessage exposes only fixed public messages, never wrapped details.
func itemErrorMessage(err error) string {
	switch {
	case errors.Is(err, core.ErrItemNotFound):
		return core.ErrItemNotFound.Error()
	case errors.Is(err, core.ErrItemAccessDenied):
		return core.ErrItemAccessDenied.Error()
	case errors.Is(err, core.ErrInvalidInput):
		return core.ErrInvalidInput.Error()
	default:
		return "internal server error"
	}
}

func writeItemError(ctx context.Context, w http.ResponseWriter, operation string, err error) {
	status := mapItemError(err)
	if status == http.StatusInternalServerError {
		// Arbitrary error strings and request fields can contain credentials or PII.
		slog.ErrorContext(ctx, "item operation failed", "operation", operation, "status", status, "error_kind", observability.ErrorKind(err))
	}
	writeError(w, status, operation+"_failed", itemErrorMessage(err))
}
