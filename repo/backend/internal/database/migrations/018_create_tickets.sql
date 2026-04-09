CREATE TABLE tickets (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    ticket_number VARCHAR(30) NOT NULL UNIQUE,
    type VARCHAR(30) NOT NULL CHECK (type IN ('seat_exception', 'delivery_exception', 'payment_issue', 'general', 'moderation_appeal')),
    subject VARCHAR(500) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'assigned', 'in_progress', 'resolved', 'closed')),
    priority VARCHAR(10) NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    assigned_to UUID REFERENCES users(id),
    created_by UUID NOT NULL REFERENCES users(id),
    related_entity_type VARCHAR(30),
    related_entity_id UUID,
    sla_response_deadline TIMESTAMPTZ,
    sla_resolution_deadline TIMESTAMPTZ,
    sla_response_met BOOLEAN,
    sla_resolution_met BOOLEAN,
    responded_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tickets_status ON tickets (status);
CREATE INDEX idx_tickets_type ON tickets (type);
CREATE INDEX idx_tickets_assigned ON tickets (assigned_to);
CREATE INDEX idx_tickets_number ON tickets (ticket_number);
CREATE INDEX idx_tickets_sla_response ON tickets (sla_response_deadline);
CREATE INDEX idx_tickets_sla_resolution ON tickets (sla_resolution_deadline);

CREATE TABLE ticket_comments (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    ticket_id UUID NOT NULL REFERENCES tickets(id),
    user_id UUID NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_comments_ticket ON ticket_comments (ticket_id);
