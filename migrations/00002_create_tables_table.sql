-- migrations/00002_create_tables_table.sql

-- +migrate Up
CREATE TABLE tables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- PostgreSQL specific for auto-generated UUIDs
    created_by INTEGER NOT NULL REFERENCES users(id),
    table_code VARCHAR(10) UNIQUE NOT NULL,   
    name VARCHAR(100), 
    restaurant_name VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'open',   -- Status of the table (e.g., 'open', 'locked', 'closed', 'paid')   
    menu_url TEXT,                      
    members INTEGER[] NOT NULL, -- Array of user IDs
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ 
);

CREATE INDEX IF NOT EXISTS idx_tables_table_code ON tables(table_code);
CREATE INDEX IF NOT EXISTS idx_tables_created_by_user_id ON tables(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_tables_status ON tables(status);

-- +migrate Down
DROP INDEX IF EXISTS idx_tables_status;
DROP INDEX IF EXISTS idx_tables_created_by_user_id;
DROP INDEX IF EXISTS idx_tables_table_code;
DROP TABLE IF EXISTS tables;
