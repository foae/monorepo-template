-- name: CreateItem :one
-- Idempotent upsert: if (name, owner_id) already exists, touch updated_at and return it.
-- Use ON CONFLICT DO NOTHING instead if you want to silently skip duplicates without returning.
INSERT INTO items (name, owner_id)
VALUES ($1, $2)
ON CONFLICT (name, owner_id) DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: GetItemByID :one
SELECT * FROM items WHERE id = $1 LIMIT 1;

-- name: ListItemsByOwner :many
SELECT * FROM items WHERE owner_id = $1 ORDER BY created_at DESC;

-- name: DeleteItemByID :exec
DELETE FROM items WHERE id = $1;
