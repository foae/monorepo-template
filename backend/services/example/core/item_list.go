package core

import (
	"context"
	"fmt"

	"go-service-template/backend/services/example/storage/postgres/sqlc"
)

// ListItems returns all items for the given owner.
func (s *Service) ListItems(ctx context.Context, ownerID string) ([]sqlc.Item, error) {
	items, err := s.db.Queries().ListItemsByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}

	if items == nil {
		items = []sqlc.Item{}
	}

	return items, nil
}
