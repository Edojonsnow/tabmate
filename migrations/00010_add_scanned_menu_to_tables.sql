-- +goose Up
ALTER TABLE tables ADD COLUMN scanned_menu TEXT;

-- +goose Down
ALTER TABLE tables DROP COLUMN scanned_menu;
