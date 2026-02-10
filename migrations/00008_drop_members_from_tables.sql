-- migrations/00008_drop_members_from_tables.sql

-- +goose Up
ALTER TABLE tables DROP COLUMN members;

-- +goose Down
ALTER TABLE tables ADD COLUMN members INTEGER[] NOT NULL DEFAULT '{}';
