-- +goose Up
ALTER TABLE users ADD COLUMN push_token TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN push_token;
