-- +goose Up
CREATE TABLE table_sync_operations (
    operation_id TEXT PRIMARY KEY,
    table_code   TEXT NOT NULL REFERENCES tables(table_code) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    applied_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_table_sync_operations_table_code ON table_sync_operations(table_code);
CREATE INDEX idx_table_sync_operations_user_id ON table_sync_operations(user_id);

-- +goose Down
DROP TABLE IF EXISTS table_sync_operations;
