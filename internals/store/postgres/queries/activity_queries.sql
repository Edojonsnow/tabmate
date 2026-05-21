-- name: InsertActivityEvent :one
INSERT INTO activity_events (
    event_type,
    actor_id,
    actor_name,
    entity_type,
    entity_code,
    entity_name,
    metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListActivityEventsForUser :many
-- Returns the 50 most recent events from all tables and splits the user belongs to.
SELECT e.*
FROM activity_events e
WHERE e.entity_code IN (
    SELECT t.table_code
    FROM tables t
    JOIN table_members tm ON tm.table_id = t.id
    WHERE tm.user_id = $1
    UNION
    SELECT s.split_code
    FROM splits s
    JOIN split_members sm ON sm.split_id = s.id
    WHERE sm.user_id = $1
)
ORDER BY e.created_at DESC
LIMIT 50;
