-- migrations/00005_create_items_table.sql

-- +goose Up
CREATE TABLE items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_code VARCHAR(10) NOT NULL REFERENCES tables(table_code) ON DELETE CASCADE,
    added_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    description TEXT,
    source VARCHAR(20) DEFAULT 'manual', -- e.g., 'manual', 'scanned'
    original_parsed_text TEXT,          -- If from a scan, the raw text parsed for this item

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_items_table_code ON items(table_code);
CREATE INDEX IF NOT EXISTS idx_items_added_by_user_id ON items(added_by_user_id);


-- +goose Down
DROP INDEX IF EXISTS idx_items_added_by_user_id;
DROP INDEX IF EXISTS idx_items_table_code;
DROP TABLE IF EXISTS items;