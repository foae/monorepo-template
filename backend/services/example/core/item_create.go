package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"monorepo-template/backend/services/example/storage/postgres/sqlc"
)

// CreateItem creates a new item for the given owner.
// Idempotent: ON CONFLICT (name, owner_id) returns the existing row.
func (s *Service) CreateItem(ctx context.Context, ownerID, name string) (sqlc.Item, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || strings.ContainsRune(name, '\x00') {
		return sqlc.Item{}, fmt.Errorf("%w: name must contain 1 to 255 valid characters", ErrInvalidInput)
	}
	if ownerID == "" || !utf8.ValidString(ownerID) || utf8.RuneCountInString(ownerID) > 255 || strings.ContainsRune(ownerID, '\x00') {
		return sqlc.Item{}, fmt.Errorf("%w: owner_id must contain 1 to 255 valid characters", ErrInvalidInput)
	}

	item, err := s.db.Queries().CreateItem(ctx, sqlc.CreateItemParams{
		Name:    name,
		OwnerID: ownerID,
	})
	if err != nil {
		return sqlc.Item{}, fmt.Errorf("create item: %w", err)
	}

	slog.InfoContext(ctx, "item created",
		"item_id", item.ID)

	return item, nil
}
