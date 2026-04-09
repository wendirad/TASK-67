CREATE TABLE waitlist (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id),
    user_id UUID NOT NULL REFERENCES users(id),
    position INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'promoted', 'canceled', 'expired')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    promoted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_waitlist_session_position ON waitlist (session_id, position);
CREATE UNIQUE INDEX idx_waitlist_session_user ON waitlist (session_id, user_id) WHERE status = 'waiting';
