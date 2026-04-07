-- name: RegisterTableSyncOperation :execrows
INSERT INTO table_sync_operations (
    operation_id,
    table_code,
    user_id
) VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (operation_id) DO NOTHING;
