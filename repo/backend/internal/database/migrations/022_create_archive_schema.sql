CREATE SCHEMA IF NOT EXISTS archive;

-- archive.orders mirrors public.orders + archived_at
CREATE TABLE archive.orders (
    id UUID NOT NULL PRIMARY KEY,
    order_number VARCHAR(30) NOT NULL,
    user_id UUID NOT NULL,
    status VARCHAR(25) NOT NULL,
    total_cents INTEGER NOT NULL,
    shipping_address_id UUID,
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
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- archive.order_items mirrors public.order_items + archived_at
CREATE TABLE archive.order_items (
    id UUID NOT NULL PRIMARY KEY,
    order_id UUID NOT NULL,
    product_id UUID NOT NULL,
    product_name VARCHAR(300) NOT NULL,
    quantity INTEGER NOT NULL,
    unit_price_cents INTEGER NOT NULL,
    total_cents INTEGER NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- archive.payments mirrors public.payments + archived_at
CREATE TABLE archive.payments (
    id UUID NOT NULL PRIMARY KEY,
    order_id UUID NOT NULL,
    payment_method VARCHAR(30) NOT NULL,
    amount_cents INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL,
    transaction_id VARCHAR(100),
    wechat_prepay_data TEXT,
    callback_signature VARCHAR(255),
    callback_received_at TIMESTAMPTZ,
    refund_id VARCHAR(100),
    refunded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- archive.tickets mirrors public.tickets + archived_at
CREATE TABLE archive.tickets (
    id UUID NOT NULL PRIMARY KEY,
    ticket_number VARCHAR(30) NOT NULL,
    type VARCHAR(30) NOT NULL,
    subject VARCHAR(500) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(20) NOT NULL,
    priority VARCHAR(10) NOT NULL,
    assigned_to UUID,
    created_by UUID NOT NULL,
    related_entity_type VARCHAR(30),
    related_entity_id UUID,
    sla_response_deadline TIMESTAMPTZ,
    sla_resolution_deadline TIMESTAMPTZ,
    sla_response_met BOOLEAN,
    sla_resolution_met BOOLEAN,
    responded_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- archive.ticket_comments mirrors public.ticket_comments + archived_at
CREATE TABLE archive.ticket_comments (
    id UUID NOT NULL PRIMARY KEY,
    ticket_id UUID NOT NULL,
    user_id UUID NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- archive.audit_logs mirrors public.audit_logs + archived_at
CREATE TABLE archive.audit_logs (
    id UUID NOT NULL PRIMARY KEY,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    performed_by UUID,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
