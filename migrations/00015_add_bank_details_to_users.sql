-- +goose Up
ALTER TABLE users
    ADD COLUMN bank_name TEXT,
    ADD COLUMN account_name TEXT,
    ADD COLUMN account_number TEXT;

-- +goose Down
ALTER TABLE users
    DROP COLUMN bank_name,
    DROP COLUMN account_name,
    DROP COLUMN account_number;
