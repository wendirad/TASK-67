CREATE TABLE payments (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id),
    payment_method VARCHAR(30) NOT NULL DEFAULT 'wechat_pay',
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'failed', 'refunded')),
    transaction_id VARCHAR(100) UNIQUE,
    wechat_prepay_data TEXT,
    callback_signature VARCHAR(255),
    callback_received_at TIMESTAMPTZ,
    refund_id VARCHAR(100),
    refunded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order ON payments (order_id);
CREATE INDEX idx_payments_status ON payments (status);
CREATE INDEX idx_payments_transaction ON payments (transaction_id);
