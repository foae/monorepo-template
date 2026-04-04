package handler

import "time"

// CreateItemRequest is the request body for POST /api/v1/items.
type CreateItemRequest struct {
	Name string `json:"name"`
}

// ItemResponse is the standard item representation in API responses.
type ItemResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
