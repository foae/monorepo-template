package handler

import (
	"errors"
	"net/http"

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
