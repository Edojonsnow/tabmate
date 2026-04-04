-- +goose Up
ALTER TABLE fixedbills RENAME TO splits;
ALTER TABLE splits RENAME COLUMN bill_code TO split_code;

ALTER TABLE bill_members RENAME TO split_members;
ALTER TABLE split_members RENAME COLUMN bill_id TO split_id;

ALTER INDEX IF EXISTS idx_fixedbills_bill_code RENAME TO idx_splits_split_code;
ALTER INDEX IF EXISTS idx_fixedbills_created_by RENAME TO idx_splits_created_by;
ALTER INDEX IF EXISTS idx_fixedbills_status RENAME TO idx_splits_status;
ALTER INDEX IF EXISTS idx_bill_members_bill_id RENAME TO idx_split_members_split_id;
ALTER INDEX IF EXISTS idx_bill_members_user_id RENAME TO idx_split_members_user_id;
ALTER INDEX IF EXISTS idx_bill_members_is_settled RENAME TO idx_split_members_is_settled;

-- +goose Down
ALTER INDEX IF EXISTS idx_split_members_is_settled RENAME TO idx_bill_members_is_settled;
ALTER INDEX IF EXISTS idx_split_members_user_id RENAME TO idx_bill_members_user_id;
ALTER INDEX IF EXISTS idx_split_members_split_id RENAME TO idx_bill_members_bill_id;
ALTER INDEX IF EXISTS idx_splits_status RENAME TO idx_fixedbills_status;
ALTER INDEX IF EXISTS idx_splits_created_by RENAME TO idx_fixedbills_created_by;
ALTER INDEX IF EXISTS idx_splits_split_code RENAME TO idx_fixedbills_bill_code;

ALTER TABLE split_members RENAME COLUMN split_id TO bill_id;
ALTER TABLE split_members RENAME TO bill_members;
ALTER TABLE splits RENAME COLUMN split_code TO bill_code;
ALTER TABLE splits RENAME TO fixedbills;
