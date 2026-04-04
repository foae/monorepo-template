package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DeleteItem deletes an item by ID with ownership validation.
// Idempotent: deleting a non-existent item returns ErrItemNotFound (handler maps to 404).
func (s *Service) DeleteItem(ctx context.Context, ownerID string, itemID pgtype.UUID) error {
	item, err := s.db.Queries().GetItemByID(ctx, itemID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrItemNotFound
		}
		return fmt.Errorf("get item for delete: %w", err)
	}

	if item.OwnerID != ownerID {
		return ErrItemAccessDenied
	}

	if err := s.db.Queries().DeleteItemByID(ctx, itemID); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}

	slog.InfoContext(ctx, "item deleted",
		"item_id", itemID, "name", item.Name, "owner_id", ownerID)

	return nil
}
