-- name: CreateFixedBill :one
INSERT INTO fixedbills (created_by, bill_code, name, description, total_amount, status)
VALUES ($1, $2, $3, $4, $5, 'open')
RETURNING *;

-- name: GetFixedBillByCode :one
SELECT * FROM fixedbills
WHERE bill_code = $1;

-- name: GetFixedBillByID :one
SELECT * FROM fixedbills
WHERE id = $1;

-- name: ListFixedBillsByUserID :many
-- Get all bills a user created
SELECT * FROM fixedbills
WHERE created_by = $1
ORDER BY created_at DESC;

-- name: UpdateFixedBillAmount :one
UPDATE fixedbills
SET total_amount = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateFixedBillStatus :one
UPDATE fixedbills
SET 
    status = $2,
    settled_at = CASE WHEN $2 = 'settled' THEN NOW() ELSE settled_at END,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteFixedBillByCode :exec
DELETE FROM fixedbills
WHERE bill_code = $1;

-- name: CountOpenFixedBills :one
SELECT COUNT(*) FROM fixedbills
WHERE status = 'open';