CREATE TABLE sessions (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    title VARCHAR(300) NOT NULL,
    description TEXT,
    coach_name VARCHAR(200),
    facility_id UUID NOT NULL REFERENCES facilities(id),
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    total_seats INTEGER NOT NULL CHECK (total_seats > 0),
    available_seats INTEGER NOT NULL CHECK (available_seats >= 0),
    registration_close_before_minutes INTEGER NOT NULL DEFAULT 120,
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('draft', 'open', 'closed', 'in_progress', 'completed', 'canceled')),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_time > start_time),
    CHECK (available_seats <= total_seats)
);

CREATE INDEX idx_sessions_start_time ON sessions (start_time);
CREATE INDEX idx_sessions_status ON sessions (status);
CREATE INDEX idx_sessions_facility_id ON sessions (facility_id);
