-- name: ListItemsInTable :many
-- Retrieves all the items in a table.
SELECT * FROM items
WHERE table_code = $1
ORDER BY created_at ASC;

-- name: ListItemsWithUserDetailsInTable :many
-- Retrieves all the items in a table with user details (username).
SELECT 
    i.id,
    i.table_code,
    i.added_by_user_id,
    i.name,
    i.price,
    i.quantity,
    i.description,
    i.source,
    i.original_parsed_text,
    i.created_at,
    i.updated_at,
    u.name AS added_by_username,
    u.email AS added_by_email
FROM items i
JOIN users u ON i.added_by_user_id = u.id
WHERE i.table_code = $1
ORDER BY i.created_at ASC;


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

-- name: UpdateItemQuantity :one
-- Updates the quantity of a single item

UPDATE items 
SET quantity = $1
WHERE id = $2
RETURNING *;


-- name: AddMenuItemsToDB :exec
-- Adds multiple items to the database.
INSERT INTO items (
    table_code,
    added_by_user_id,
    name,
    price,
    quantity,
    description,
    source,
    original_parsed_text
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
);
