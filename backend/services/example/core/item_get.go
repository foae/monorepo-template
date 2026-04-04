package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"monorepo-template/backend/services/example/storage/postgres/sqlc"
)

// GetItem retrieves an item by ID with ownership validation.
func (s *Service) GetItem(ctx context.Context, ownerID string, itemID pgtype.UUID) (sqlc.Item, error) {
	item, err := s.db.Queries().GetItemByID(ctx, itemID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return sqlc.Item{}, ErrItemNotFound
		}
		return sqlc.Item{}, fmt.Errorf("get item: %w", err)
	}

	if item.OwnerID != ownerID {
		return sqlc.Item{}, ErrItemAccessDenied
	}

	return item, nil
}
