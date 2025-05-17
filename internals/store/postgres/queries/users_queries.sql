
-- name: CreateUser :one
INSERT INTO users (id, cognito_sub, email)
VALUES ($1, $2, $3)
RETURNING *;


-- name: FindUser :one
SELECT * FROM users
WHERE email = $1;

-- name: ListAllUsers :many
SELECT * FROM users;