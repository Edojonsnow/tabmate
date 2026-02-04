-- +goose Up
CREATE TABLE bill_members (
    bill_id UUID NOT NULL REFERENCES fixedbills(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- How much this person owes (calculated: total_amount / member_count)
    amount_owed DECIMAL(10, 2) NOT NULL,
    
    -- Payment tracking
    is_settled BOOLEAN NOT NULL DEFAULT FALSE,
    settled_at TIMESTAMPTZ,
    
    -- Role: 'host' (creator who paid) or 'guest' (owes money)
    role VARCHAR(20) NOT NULL DEFAULT 'guest',
    
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (bill_id, user_id)
);

CREATE INDEX idx_bill_members_user_id ON bill_members(user_id);
CREATE INDEX idx_bill_members_bill_id ON bill_members(bill_id);
CREATE INDEX idx_bill_members_is_settled ON bill_members(is_settled);

-- +goose Down
DROP INDEX IF EXISTS idx_bill_members_is_settled;
DROP INDEX IF EXISTS idx_bill_members_bill_id;
DROP INDEX IF EXISTS idx_bill_members_user_id;
DROP TABLE IF EXISTS bill_members;