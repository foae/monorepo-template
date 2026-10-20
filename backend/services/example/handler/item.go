package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxCreateItemBodyBytes = 1 << 20

// HandleCreateItem handles POST /api/v1/items.
func (h *Handler) HandleCreateItem() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := extractOwnerID(r)
		if ownerID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "X-Owner-Id header is required")
			return
		}

		var req CreateItemRequest
		r.Body = http.MaxBytesReader(w, r.Body, maxCreateItemBodyBytes)
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&req)
		if err == nil {
			var extra json.RawMessage
			if trailingErr := decoder.Decode(&extra); trailingErr != io.EOF {
				if trailingErr == nil {
					err = errors.New("multiple JSON values")
				} else {
					err = trailingErr
				}
			}
		}
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "request_body_too_large", "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		item, err := h.svc.CreateItem(r.Context(), ownerID, req.Name)
		if err != nil {
			writeItemError(r.Context(), w, "create_item", err)
			return
		}

		writeJSON(w, http.StatusCreated, itemToResponse(item))
	}
}

// HandleGetItem handles GET /api/v1/items/{id}.
func (h *Handler) HandleGetItem() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := extractOwnerID(r)
		if ownerID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "X-Owner-Id header is required")
			return
		}

		itemID, err := parseUUID(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid item ID")
			return
		}

		item, err := h.svc.GetItem(r.Context(), ownerID, itemID)
		if err != nil {
			writeItemError(r.Context(), w, "get_item", err)
			return
		}

		writeJSON(w, http.StatusOK, itemToResponse(item))
	}
}

// HandleListItems handles GET /api/v1/items.
func (h *Handler) HandleListItems() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := extractOwnerID(r)
		if ownerID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "X-Owner-Id header is required")
			return
		}

		items, err := h.svc.ListItems(r.Context(), ownerID)
		if err != nil {
			writeItemError(r.Context(), w, "list_items", err)
			return
		}

		resp := make([]ItemResponse, len(items))
		for i, item := range items {
			resp[i] = itemToResponse(item)
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// HandleDeleteItem handles DELETE /api/v1/items/{id}.
func (h *Handler) HandleDeleteItem() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := extractOwnerID(r)
		if ownerID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "X-Owner-Id header is required")
			return
		}

		itemID, err := parseUUID(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid item ID")
			return
		}

		if err := h.svc.DeleteItem(r.Context(), ownerID, itemID); err != nil {
			writeItemError(r.Context(), w, "delete_item", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// parseUUID parses a UUID string into a pgtype.UUID.
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return u, err
	}
	return u, nil
}
