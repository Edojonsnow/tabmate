-- name: ListItemsInTable :many
-- Retrieves all the items in a table.
SELECT * FROM items
WHERE table_id = $1
ORDER BY created_at ASC;


-- name: DeleteItemFromTable :exec
-- Remove an item from a table
DELETE FROM items
WHERE id = $1;
