-- migrations/00003_create_table_members_table.sql

-- +migrate Up
CREATE TABLE table_members (
    table_id UUID NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    is_settled BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (table_id, user_id) -- Ensures a user can only be in a table once
);

CREATE INDEX IF NOT EXISTS idx_table_members_user_id ON table_members(user_id);
CREATE INDEX IF NOT EXISTS idx_table_members_table_id ON table_members(table_id);


-- +migrate Down
DROP INDEX IF EXISTS idx_table_members_user_id;
DROP INDEX IF EXISTS idx_table_members_table_id;
DROP TABLE IF EXISTS table_members;