-- +goose Up

-- New columns on splits for receipt-based splitting
ALTER TABLE splits
  ADD COLUMN tax_amount    NUMERIC  NOT NULL DEFAULT 0,
  ADD COLUMN tip_amount    NUMERIC  NOT NULL DEFAULT 0,
  ADD COLUMN tip_is_shared BOOLEAN  NOT NULL DEFAULT FALSE,
  ADD COLUMN split_type    TEXT     NOT NULL DEFAULT 'simple';

-- Items extracted from a receipt and attached to a split
CREATE TABLE split_items (
  id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  split_id         UUID        NOT NULL REFERENCES splits(id) ON DELETE CASCADE,
  name             TEXT        NOT NULL,
  price            NUMERIC     NOT NULL,
  quantity         INT         NOT NULL DEFAULT 1,
  remaining_qty    INT         NOT NULL DEFAULT 1,
  added_by_user_id UUID        REFERENCES users(id) ON DELETE SET NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_split_items_split_id ON split_items(split_id);

-- Per-member item claims (who claimed how many units of each item)
CREATE TABLE split_item_claims (
  split_item_id       UUID        NOT NULL REFERENCES split_items(id) ON DELETE CASCADE,
  claimed_by_user_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  quantity_claimed    INT         NOT NULL DEFAULT 1,
  claimed_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (split_item_id, claimed_by_user_id)
);

CREATE INDEX idx_split_item_claims_item_id ON split_item_claims(split_item_id);
CREATE INDEX idx_split_item_claims_user_id ON split_item_claims(claimed_by_user_id);

-- +goose Down
DROP TABLE IF EXISTS split_item_claims;
DROP TABLE IF EXISTS split_items;
ALTER TABLE splits
  DROP COLUMN IF EXISTS split_type,
  DROP COLUMN IF EXISTS tip_is_shared,
  DROP COLUMN IF EXISTS tip_amount,
  DROP COLUMN IF EXISTS tax_amount;
