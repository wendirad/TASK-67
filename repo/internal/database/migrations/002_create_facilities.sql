CREATE TABLE facilities (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    checkin_mode VARCHAR(20) NOT NULL DEFAULT 'staff_qr' CHECK (checkin_mode IN ('staff_qr', 'staff_qr_bluetooth')),
    bluetooth_beacon_id VARCHAR(100),
    bluetooth_beacon_range_meters INTEGER DEFAULT 10,
    kiosk_device_token VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
