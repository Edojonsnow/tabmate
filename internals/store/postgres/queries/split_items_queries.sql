-- name: AddSplitItem :one
INSERT INTO split_items (split_id, name, price, quantity, remaining_qty, added_by_user_id)
VALUES ($1, $2, $3, $4, $4, $5)
RETURNING *;

-- name: GetSplitItem :one
SELECT * FROM split_items WHERE id = $1;

-- name: ListSplitItems :many
SELECT * FROM split_items WHERE split_id = $1 ORDER BY created_at ASC;

-- name: DeleteSplitItem :exec
DELETE FROM split_items WHERE id = $1;

-- name: UpdateSplitItemRemainingQty :one
UPDATE split_items SET remaining_qty = $2 WHERE id = $1 RETURNING *;

-- name: CountUnclaimedSplitItems :one
SELECT COUNT(*) FROM split_items WHERE split_id = $1 AND remaining_qty > 0;

-- name: AddSplitItemClaim :one
INSERT INTO split_item_claims (split_item_id, claimed_by_user_id, quantity_claimed)
VALUES ($1, $2, $3)
ON CONFLICT (split_item_id, claimed_by_user_id)
DO UPDATE SET quantity_claimed = EXCLUDED.quantity_claimed, claimed_at = NOW()
RETURNING *;

-- name: GetSplitItemClaim :one
SELECT * FROM split_item_claims
WHERE split_item_id = $1 AND claimed_by_user_id = $2;

-- name: ListClaimsForItem :many
SELECT
    sic.split_item_id,
    sic.claimed_by_user_id,
    sic.quantity_claimed,
    sic.claimed_at,
    u.name AS user_name
FROM split_item_claims sic
JOIN users u ON sic.claimed_by_user_id = u.id
WHERE sic.split_item_id = $1;

-- name: ListClaimsForSplit :many
SELECT
    sic.split_item_id,
    sic.claimed_by_user_id,
    sic.quantity_claimed,
    sic.claimed_at,
    si.name  AS item_name,
    si.price AS item_price,
    u.name   AS user_name
FROM split_item_claims sic
JOIN split_items si ON sic.split_item_id = si.id
JOIN users u ON sic.claimed_by_user_id = u.id
WHERE si.split_id = $1;

-- name: DeleteSplitItemClaim :exec
DELETE FROM split_item_claims
WHERE split_item_id = $1 AND claimed_by_user_id = $2;

-- name: DeleteAllSplitItems :exec
DELETE FROM split_items WHERE split_id = $1;

-- name: UpdateSplitTotalAmount :one
UPDATE splits SET total_amount = $2, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: UpdateSplitReceiptDetails :one
UPDATE splits
SET
    tax_amount    = $2,
    tip_amount    = $3,
    tip_is_shared = $4,
    total_amount  = $5,
    split_type    = 'receipt',
    updated_at    = NOW()
WHERE id = $1
RETURNING *;
