-- +goose Up
CREATE TABLE fixedbills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bill_code VARCHAR(10) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,  -- Optional: "Dinner at XYZ", "Trip to ABC"
    
    -- The key difference: fixed total amount
    total_amount DECIMAL(10, 2) NOT NULL,
    
    -- Status: 'open', 'locked', 'settled', 'cancelled'
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ  -- When bill was fully paid
);

-- Indexes
CREATE INDEX idx_fixedbills_bill_code ON fixedbills(bill_code);
CREATE INDEX idx_fixedbills_created_by ON fixedbills(created_by);
CREATE INDEX idx_fixedbills_status ON fixedbills(status);

-- +goose Down
DROP INDEX IF EXISTS idx_fixedbills_status;
DROP INDEX IF EXISTS idx_fixedbills_created_by;
DROP INDEX IF EXISTS idx_fixedbills_bill_code;
DROP TABLE IF EXISTS fixedbills;