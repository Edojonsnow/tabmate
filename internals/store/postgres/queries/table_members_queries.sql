-- name: AddUserToTable :one
-- Adds a user to a table with an optional role, defaulting is_settled to false.
INSERT INTO table_members (
    table_id,
    user_id,
    role
    -- joined_at and is_settled will use their DEFAULT values
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: GetTableMember :one
-- Retrieves a specific membership record by table_id and user_id.
SELECT * FROM table_members
WHERE table_id = $1 AND user_id = $2;

-- name: ListMembersByTableID :many
-- Retrieves all membership records for a specific table_id.
SELECT * FROM table_members
WHERE table_id = $1
ORDER BY joined_at ASC;

-- -- name: ListTablesByUserID :many
-- -- Retrieves all membership records for a specific user_id.
-- SELECT * FROM table_members
-- WHERE user_id = $1
-- ORDER BY joined_at DESC;

-- name: GetMemberRoleInTable :one
-- Retrieves the role of a specific user in a specific table.
SELECT role FROM table_members
WHERE table_id = $1 AND user_id = $2;

-- name: CountMembersInTable :one
-- Counts the number of members in a specific table.
SELECT COUNT(*) FROM table_members
WHERE table_id = $1; 

-- name: CheckIfUserIsMember :one
-- Checks if a specific user is a member of a specific table.
SELECT EXISTS(
    SELECT 1 FROM table_members
    WHERE table_id = $1 AND user_id = $2
);

-- name: ListSettledMembersInTable :many
-- Retrieves all members of a table_id where is_settled is true.
SELECT * FROM table_members
WHERE table_id = $1 AND is_settled = TRUE
ORDER BY joined_at ASC;

-- name: ListUnsettledMembersInTable :many
-- Retrieves all members of a table_id where is_settled is false.
SELECT * FROM table_members
WHERE table_id = $1 AND is_settled = FALSE
ORDER BY joined_at ASC;

-- name: GetTableMembershipDetailsForUser :many
-- Retrieves all membership details for a specific user across all tables.
SELECT * FROM table_members
WHERE user_id = $1
ORDER BY table_id, joined_at DESC;

-- name: UpdateMemberRoleInTable :one
-- Updates the role of a user within a specific table.
UPDATE table_members
SET role = $3
WHERE table_id = $1 AND user_id = $2
RETURNING *;

-- name: SetMemberSettledStatus :one
-- Updates the is_settled status for a user in a specific table.
UPDATE table_members
SET is_settled = $3
WHERE table_id = $1 AND user_id = $2
RETURNING *;

-- name: MarkAllMembersInTableAsSettled :many
-- Sets is_settled to true for all members of a specific table.
-- Returns all updated member rows.
UPDATE table_members
SET is_settled = TRUE
WHERE table_id = $1
RETURNING *;

-- name: RemoveUserFromTable :exec
-- Removes a user from a specific table.
DELETE FROM table_members
WHERE table_id = $1 AND user_id = $2;

-- name: ListMembersWithUserDetailsByTableID :many
-- Retrieves all members of a specific table_id and include their user details.
SELECT
    tm.table_id,
    tm.user_id,
    tm.joined_at,
    tm.role,
    tm.is_settled,
    u.email AS user_email,
    u.name AS user_name,
    u.profile_picture_url AS user_profile_picture_url
FROM table_members tm
JOIN users u ON tm.user_id = u.id
WHERE tm.table_id = $1
ORDER BY tm.joined_at ASC;

-- name: ListTablesWithMembershipStatusForUser :many
-- For a specific user, list all tables they are a member of,
-- along with their role and is_settled status in each, and table details.
SELECT
    t.id AS table_id,
    t.table_code,
    t.name AS table_name,
    t.restaurant_name,
    t.status AS table_status,
    tm.role AS user_role_in_table,
    tm.is_settled AS user_is_settled_in_table,
    tm.joined_at
FROM table_members tm
JOIN tables t ON tm.table_id = t.id
WHERE tm.user_id = $1
ORDER BY t.created_at DESC, tm.joined_at DESC; 