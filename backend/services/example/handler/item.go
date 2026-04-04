package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// HandleCreateItem handles POST /api/v1/items.
func (h *Handler) HandleCreateItem() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := extractOwnerID(r)
		if ownerID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "X-Owner-Id header is required")
			return
		}

		var req CreateItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		item, err := h.svc.CreateItem(r.Context(), ownerID, req.Name)
		if err != nil {
			status := mapItemError(err)
			slog.Error("failed to create item", "error", err, "owner_id", ownerID, "name", req.Name)
			writeError(w, status, "create_item_failed", err.Error())
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
			status := mapItemError(err)
			writeError(w, status, "get_item_failed", err.Error())
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
			slog.Error("failed to list items", "error", err, "owner_id", ownerID)
			writeError(w, http.StatusInternalServerError, "list_items_failed", err.Error())
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
			status := mapItemError(err)
			slog.Error("failed to delete item", "error", err, "owner_id", ownerID, "item_id", itemID)
			writeError(w, status, "delete_item_failed", err.Error())
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
