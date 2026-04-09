CREATE TABLE seats (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id),
    seat_number INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'reserved', 'occupied', 'on_break', 'released')),
    assigned_user_id UUID REFERENCES users(id),
    assigned_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_seats_session_number ON seats (session_id, seat_number);
CREATE INDEX idx_seats_session_status ON seats (session_id, status);
CREATE INDEX idx_seats_assigned_user ON seats (assigned_user_id);
