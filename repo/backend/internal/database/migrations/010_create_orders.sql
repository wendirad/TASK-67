CREATE TABLE orders (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    order_number VARCHAR(30) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id),
    status VARCHAR(25) NOT NULL DEFAULT 'created' CHECK (status IN ('created', 'pending_payment', 'paid', 'processing', 'shipped', 'delivered', 'completed', 'closed', 'refund_pending', 'refunded')),
    total_cents INTEGER NOT NULL CHECK (total_cents >= 0),
    shipping_address_id UUID REFERENCES addresses(id),
    ship_to_recipient VARCHAR(200),
    ship_to_phone VARCHAR(20),
    ship_to_line1 VARCHAR(500),
    ship_to_line2 VARCHAR(500),
    ship_to_city VARCHAR(100),
    ship_to_province VARCHAR(100),
    ship_to_postal_code VARCHAR(20),
    payment_deadline TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    close_reason VARCHAR(500),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user ON orders (user_id);
CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_number ON orders (order_number);
CREATE INDEX idx_orders_payment_deadline ON orders (payment_deadline);
