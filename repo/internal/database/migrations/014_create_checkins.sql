CREATE TABLE check_ins (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    registration_id UUID NOT NULL REFERENCES registrations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    session_id UUID NOT NULL REFERENCES sessions(id),
    seat_id UUID NOT NULL REFERENCES seats(id),
    confirmed_by UUID NOT NULL REFERENCES users(id),
    method VARCHAR(20) NOT NULL DEFAULT 'qr_scan' CHECK (method IN ('qr_scan', 'qr_scan_bluetooth')),
    checked_in_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    checked_out_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'on_break', 'completed', 'released')),
    break_count INTEGER NOT NULL DEFAULT 0,
    total_break_minutes INTEGER NOT NULL DEFAULT 0,
    last_break_start TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_checkins_registration ON check_ins (registration_id);
CREATE INDEX idx_checkins_session ON check_ins (session_id);
CREATE INDEX idx_checkins_user ON check_ins (user_id);
CREATE INDEX idx_checkins_status ON check_ins (status);
CREATE INDEX idx_checkins_confirmed_by ON check_ins (confirmed_by);
