CREATE TABLE registrations (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    session_id UUID NOT NULL REFERENCES sessions(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'registered', 'waitlisted', 'canceled', 'completed', 'no_show')),
    registered_at TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,
    cancel_reason VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_reg_user_session ON registrations (user_id, session_id);
CREATE INDEX idx_reg_session_status ON registrations (session_id, status);
CREATE INDEX idx_reg_status ON registrations (status);
