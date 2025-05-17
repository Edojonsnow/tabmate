
-- name: CreateTable :one
INSERT INTO tables  (id, created_by, table_code, name, restaurant_name, status, menu_url, members, closed_at )
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9 )
RETURNING *;


