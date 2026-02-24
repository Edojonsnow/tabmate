-- name: CreateUser :one
INSERT INTO users (name, profile_picture_url, cognito_sub,  email)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByCognitoSub :one
SELECT * FROM users
WHERE cognito_sub = $1;

-- name: ListAllUsers :many
SELECT * FROM users;

-- name: CheckIfEmailExists :one
SELECT EXISTS (
    SELECT 1 FROM users
    WHERE email = $1
);

-- name: CheckIfCognitoSubExists :one
SELECT EXISTS (
    SELECT 1 FROM users
    WHERE email = $1
);

-- name: UpdateUserName :one
-- Updates the name of a user given their ID and returns the updated user row.
UPDATE users
SET
    name = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING *;

-- name: UpdateUserProfilePictureURL :one
UPDATE users
SET
    profile_picture_url = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING *;

-- name: UpdateUserEmail :one
UPDATE users
SET
    email = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING *;

-- name: DeleteUserByID :exec
DELETE FROM users
WHERE id = $1;

-- name: DeleteUserByCognitoSub :exec
DELETE FROM users
WHERE cognito_sub = $1;

-- name: SearchUsersByName :many
SELECT id, name, email, profile_picture_url FROM users
WHERE name ILIKE '%' || $1 || '%'
  AND id != $2
ORDER BY name ASC
LIMIT 10;

