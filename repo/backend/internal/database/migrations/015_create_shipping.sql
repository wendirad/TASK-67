CREATE TABLE shipping_records (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id),
    tracking_number VARCHAR(100),
    carrier VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'shipped', 'in_transit', 'delivered', 'exception')),
    shipped_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    proof_type VARCHAR(20) CHECK (proof_type IN ('signature', 'acknowledgment') OR proof_type IS NULL),
    proof_data TEXT,
    exception_notes TEXT,
    handled_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_shipping_order ON shipping_records (order_id);
CREATE INDEX idx_shipping_status ON shipping_records (status);
