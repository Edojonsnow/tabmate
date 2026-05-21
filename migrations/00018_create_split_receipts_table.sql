-- +goose Up
CREATE TABLE split_receipts (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  split_id          UUID        NOT NULL UNIQUE REFERENCES splits(id) ON DELETE CASCADE,
  object_key        TEXT        NOT NULL,
  image_url         TEXT        NOT NULL,
  media_type        TEXT        NOT NULL DEFAULT 'image/jpeg',
  original_filename TEXT,
  created_by        UUID        REFERENCES users(id) ON DELETE SET NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_split_receipts_split_id ON split_receipts(split_id);

-- +goose Down
DROP INDEX IF EXISTS idx_split_receipts_split_id;
DROP TABLE IF EXISTS split_receipts;
