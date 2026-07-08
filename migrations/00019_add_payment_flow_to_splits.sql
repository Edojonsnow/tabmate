-- +goose Up
ALTER TABLE split_members ADD COLUMN payment_status TEXT NOT NULL DEFAULT 'unpaid';
ALTER TABLE splits ADD COLUMN payment_instructions TEXT;

-- +goose Down
ALTER TABLE split_members DROP COLUMN payment_status;
ALTER TABLE splits DROP COLUMN payment_instructions;
