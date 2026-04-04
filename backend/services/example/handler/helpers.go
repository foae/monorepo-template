package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"monorepo-template/backend/services/example/storage/postgres/sqlc"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, errType, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   errType,
		Message: message,
	})
}

// extractOwnerID reads the owner identity from the X-Owner-Id header.
func extractOwnerID(r *http.Request) string {
	return r.Header.Get("X-Owner-Id")
}

// itemToResponse converts a sqlc.Item to an API response.
func itemToResponse(item sqlc.Item) ItemResponse {
	return ItemResponse{
		ID:        uuidToString(item.ID),
		Name:      item.Name,
		OwnerID:   item.OwnerID,
		CreatedAt: item.CreatedAt.Time,
		UpdatedAt: item.UpdatedAt.Time,
	}
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
