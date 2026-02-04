-- name: AddUserToBill :one
INSERT INTO bill_members (bill_id, user_id, amount_owed, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetBillMember :one
SELECT * FROM bill_members
WHERE bill_id = $1 AND user_id = $2;

-- name: ListBillMembersByBillID :many
SELECT * FROM bill_members
WHERE bill_id = $1
ORDER BY joined_at ASC;

-- name: ListBillMembersWithUserDetails :many
-- Get all members of a bill with their user info
SELECT
    bm.bill_id,
    bm.user_id,
    bm.amount_owed,
    bm.is_settled,
    bm.settled_at,
    bm.role,
    bm.joined_at,
    u.email AS user_email,
    u.name AS user_name,
    u.profile_picture_url AS user_profile_picture_url
FROM bill_members bm
JOIN users u ON bm.user_id = u.id
WHERE bm.bill_id = $1
ORDER BY bm.joined_at ASC;

-- name: ListBillsForUser :many
-- Get all bills a user is a member of
SELECT
    fb.id AS bill_id,
    fb.bill_code,
    fb.name AS bill_name,
    fb.total_amount,
    fb.status AS bill_status,
    fb.created_by as bill_creator,
    bm.role AS user_role_in_bill,
    bm.amount_owed,
    bm.is_settled AS user_is_settled,
    bm.joined_at
FROM bill_members bm
JOIN fixedbills fb ON bm.bill_id = fb.id
WHERE bm.user_id = $1
ORDER BY fb.created_at DESC;

-- name: UpdateBillMemberSettledStatus :one
UPDATE bill_members
SET 
    is_settled = $3,
    settled_at = CASE WHEN $3 = TRUE THEN NOW() ELSE NULL END
WHERE bill_id = $1 AND user_id = $2
RETURNING *;

-- name: CountUnsettledBillMembers :one
SELECT COUNT(*) FROM bill_members
WHERE bill_id = $1 AND is_settled = FALSE;

-- name: RemoveUserFromBill :exec
DELETE FROM bill_members
WHERE bill_id = $1 AND user_id = $2;

-- name: RecalculateBillSplitForAllMembers :exec
-- When someone joins/leaves, recalculate everyone's amount_owed
UPDATE bill_members bm
SET amount_owed = (
    SELECT fb.total_amount / COUNT(*)
    FROM bill_members bm2
    WHERE bm2.bill_id = bm.bill_id
)
FROM fixedbills fb
WHERE bm.bill_id = fb.id AND bm.bill_id = $1;