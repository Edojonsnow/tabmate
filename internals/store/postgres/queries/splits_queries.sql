-- name: CreateSplit :one
INSERT INTO splits (created_by, split_code, name, description, total_amount, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetSplitByCode :one
SELECT * FROM splits WHERE split_code = $1;

-- name: GetSplitByID :one
SELECT * FROM splits WHERE id = $1;

-- name: ListSplitsByUserID :many
SELECT * FROM splits WHERE created_by = $1 ORDER BY created_at DESC;

-- name: UpdateSplitAmount :one
UPDATE splits SET total_amount = $2, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: UpdateSplitStatus :one
UPDATE splits
SET
    status = $2,
    settled_at = CASE WHEN $2 = 'settled' THEN NOW() ELSE settled_at END,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteSplitByCode :exec
DELETE FROM splits WHERE split_code = $1;

-- name: CountOpenSplits :one
SELECT COUNT(*) FROM splits WHERE status = 'open';
