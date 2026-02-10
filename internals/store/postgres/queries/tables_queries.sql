-- name: CreateTable :one
INSERT INTO tables  ( created_by, table_code, name, restaurant_name, status, menu_url )
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTableByID :one
SELECT * FROM tables
WHERE id = $1;

-- name: GetTableByCode :one 
SELECT * FROM tables
WHERE table_code = $1;


-- name: ListTablesByUserID :many
SELECT * FROM tables
WHERE created_by = $1
ORDER BY created_at DESC;

-- name: ListTablesByStatus :many
SELECT * FROM tables
WHERE status = $1
ORDER BY created_at DESC;



-- name: CheckIfTableCodeExists :one
SELECT EXISTS(SELECT 1 FROM tables WHERE table_code = $1);

-- name: SearchTablesByNameOrRestaurant :many
SELECT * FROM tables
WHERE
    (name ILIKE '%' || $1 || '%' OR restaurant_name ILIKE '%' || $1 || '%')
    AND status = 'open' 
ORDER BY created_at DESC;

-- name: GetAllTableCodes :many
SELECT table_code FROM tables;


-- name: UpdateTableName :one
UPDATE tables
SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateTableRestaurantName :one
UPDATE tables
SET restaurant_name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateTableStatus :one
UPDATE tables
SET
    status = $2,
    closed_at = CASE WHEN $2 IN ('closed', 'paid') THEN NOW() ELSE closed_at END, -- Set closed_at if status changes to closed/paid
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateTableMenuURL :one
UPDATE tables
SET menu_url = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteTableByID :exec
DELETE FROM tables
WHERE id = $1;

-- name: DeleteTableByCode :exec
DELETE FROM tables
WHERE table_code = $1;

-- name: CountOpenTables :one
SELECT COUNT(*) FROM tables
WHERE status = 'open';

-- name: UpdateTableVat :one
UPDATE tables
SET vat = $2, updated_at = NOW()
WHERE table_code = $1
RETURNING *;