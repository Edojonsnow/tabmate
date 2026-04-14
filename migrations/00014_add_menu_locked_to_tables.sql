-- +goose Up
ALTER TABLE tables ADD COLUMN menu_locked BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE tables DROP COLUMN menu_locked;
