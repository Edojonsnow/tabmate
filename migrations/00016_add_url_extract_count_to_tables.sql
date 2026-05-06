-- +goose Up
ALTER TABLE tables ADD COLUMN url_extract_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE tables DROP COLUMN url_extract_count;
