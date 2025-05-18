-- migrations/00002_create_users_table.sql

-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- PostgreSQL specific for auto-generated UUIDs
    cognito_sub VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add an index on cognito_sub for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_cognito_sub ON users(cognito_sub);
-- Add an index on email for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);


-- +goose Down
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_cognito_sub;
DROP TABLE IF EXISTS users;