package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"monorepo-template/backend/services/example/storage/postgres/sqlc"
)

// CreateItem creates a new item for the given owner.
// Idempotent: ON CONFLICT (name, owner_id) returns the existing row.
func (s *Service) CreateItem(ctx context.Context, ownerID, name string) (sqlc.Item, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return sqlc.Item{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if ownerID == "" {
		return sqlc.Item{}, fmt.Errorf("%w: owner_id is required", ErrInvalidInput)
	}

	item, err := s.db.Queries().CreateItem(ctx, sqlc.CreateItemParams{
		Name:    name,
		OwnerID: ownerID,
	})
	if err != nil {
		return sqlc.Item{}, fmt.Errorf("create item: %w", err)
	}

	slog.InfoContext(ctx, "item created",
		"item_id", item.ID, "name", name, "owner_id", ownerID)

	return item, nil
}
