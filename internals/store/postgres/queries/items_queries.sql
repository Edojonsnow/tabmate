-- name: ListItemsInTable :many
-- Retrieves all the items in a table.
SELECT * FROM items
WHERE table_code = $1
ORDER BY created_at ASC;


-- name: DeleteItemFromTable :exec
-- Remove an item from a table
DELETE FROM items
WHERE id = $1;


-- name: AddItemToTable :one
-- Adds a single item to a table.
INSERT INTO items (
    table_code,
    added_by_user_id,
    name,
    price,
    quantity,
    description,
    original_parsed_text
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING *;
