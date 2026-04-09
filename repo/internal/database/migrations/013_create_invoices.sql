CREATE TABLE invoices (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    invoice_number VARCHAR(30) NOT NULL UNIQUE,
    order_id UUID NOT NULL REFERENCES orders(id),
    user_id UUID NOT NULL REFERENCES users(id),
    type VARCHAR(20) NOT NULL DEFAULT 'invoice' CHECK (type IN ('invoice', 'e_voucher', 'credit_note')),
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'issued', 'voided')),
    amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
    tax_cents INTEGER NOT NULL DEFAULT 0 CHECK (tax_cents >= 0),
    issued_at TIMESTAMPTZ,
    voided_at TIMESTAMPTZ,
    void_reason VARCHAR(500),
    refund_invoice_id UUID REFERENCES invoices(id),
    metadata TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoices_order ON invoices (order_id);
CREATE INDEX idx_invoices_user ON invoices (user_id);
CREATE UNIQUE INDEX idx_invoices_number ON invoices (invoice_number);
CREATE INDEX idx_invoices_status ON invoices (status);
