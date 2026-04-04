-- name: AddUserToSplit :one
INSERT INTO split_members (split_id, user_id, amount_owed, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSplitMember :one
SELECT * FROM split_members
WHERE split_id = $1 AND user_id = $2;

-- name: ListSplitMembersBySplitID :many
SELECT * FROM split_members
WHERE split_id = $1
ORDER BY joined_at ASC;

-- name: ListSplitMembersWithUserDetails :many
-- Get all members of a split with their user info
SELECT
    sm.split_id,
    sm.user_id,
    sm.amount_owed,
    sm.is_settled,
    sm.settled_at,
    sm.role,
    sm.joined_at,
    u.email AS user_email,
    u.name AS user_name,
    u.profile_picture_url AS user_profile_picture_url
FROM split_members sm
JOIN users u ON sm.user_id = u.id
WHERE sm.split_id = $1
ORDER BY sm.joined_at ASC;

-- name: ListSplitsForUser :many
-- Get all splits a user is a member of
SELECT
    s.id AS split_id,
    s.split_code,
    s.name AS split_name,
    s.total_amount,
    s.status AS split_status,
    s.created_by AS split_creator,
    sm.role AS user_role_in_split,
    sm.amount_owed,
    sm.is_settled AS user_is_settled,
    sm.joined_at
FROM split_members sm
JOIN splits s ON sm.split_id = s.id
WHERE sm.user_id = $1
ORDER BY s.created_at DESC;

-- name: UpdateSplitMemberSettledStatus :one
UPDATE split_members
SET
    is_settled = $3,
    settled_at = CASE WHEN $3 = TRUE THEN NOW() ELSE NULL END
WHERE split_id = $1 AND user_id = $2
RETURNING *;

-- name: CountUnsettledSplitMembers :one
SELECT COUNT(*) FROM split_members
WHERE split_id = $1 AND is_settled = FALSE;

-- name: RemoveUserFromSplit :exec
DELETE FROM split_members
WHERE split_id = $1 AND user_id = $2;

-- name: RecalculateSplitForAllMembers :exec
-- When someone joins/leaves, recalculate everyone's amount_owed
UPDATE split_members sm
SET amount_owed = (
    SELECT s.total_amount / COUNT(*)
    FROM split_members sm2
    WHERE sm2.split_id = sm.split_id
)
FROM splits s
WHERE sm.split_id = s.id AND sm.split_id = $1;
