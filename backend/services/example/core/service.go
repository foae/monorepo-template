package core

import (
	"context"
	"errors"
	"fmt"

	"monorepo-template/backend/services/example/storage/postgres"
)

// Sentinel errors for the core layer. Handler maps these to HTTP status codes.
var (
	ErrItemNotFound     = errors.New("item not found")
	ErrItemAccessDenied = errors.New("item does not belong to this owner")
	ErrInvalidInput     = errors.New("invalid input")
)

// Service encapsulates all dependencies needed for business logic execution.
type Service struct {
	db *postgres.Client
}

// New creates a new service instance with all dependencies.
func New(db *postgres.Client) (*Service, error) {
	ctx := context.Background()
	if err := db.DB().Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping DB: %w", err)
	}

	return &Service{
		db: db,
	}, nil
}

// Close releases all service resources.
func (s *Service) Close() {
	s.db.Close()
}
