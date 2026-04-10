-- Seed data: non-credential reference data only.
-- User accounts are bootstrapped via the entrypoint, not via migration.

-- Facilities
INSERT INTO facilities (id, name, checkin_mode, bluetooth_beacon_id, bluetooth_beacon_range_meters)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Studio A', 'staff_qr', NULL, NULL),
    ('a0000000-0000-0000-0000-000000000002', 'Studio B', 'staff_qr_bluetooth', 'beacon-studio-b-001', 10),
    ('a0000000-0000-0000-0000-000000000003', 'Wellness Room', 'staff_qr', NULL, NULL)
ON CONFLICT (name) DO NOTHING;

-- Products
INSERT INTO products (id, name, description, category, price_cents, stock_quantity, is_shippable, status)
VALUES
    ('b0000000-0000-0000-0000-000000000001', 'Water Bottle', 'Stainless steel 750ml water bottle', 'Accessories', 1599, 50, true, 'active'),
    ('b0000000-0000-0000-0000-000000000002', 'Yoga Mat', 'Premium non-slip yoga mat', 'Equipment', 2999, 30, true, 'active'),
    ('b0000000-0000-0000-0000-000000000003', 'Resistance Band Set', 'Set of 5 resistance bands', 'Equipment', 1999, 25, true, 'active'),
    ('b0000000-0000-0000-0000-000000000004', 'Protein Shake', 'Chocolate protein shake 500ml', 'Nutrition', 499, 100, false, 'active'),
    ('b0000000-0000-0000-0000-000000000005', 'Gym Towel', 'Quick-dry microfiber gym towel', 'Accessories', 899, 40, true, 'active')
ON CONFLICT DO NOTHING;

-- Config entries (default operational parameters)
INSERT INTO config_entries (key, value, description)
VALUES
    ('session.break_max_count', '2', 'Maximum number of breaks allowed per session'),
    ('session.break_max_minutes', '10', 'Maximum duration per break in minutes'),
    ('session.noshow_minutes', '10', 'Minutes past session start before marking no-show'),
    ('session.reg_close_default_minutes', '120', 'Default registration close time before session start'),
    ('session.occupancy_max_minutes', '180', 'Minutes of continuous occupancy before generating an exception ticket'),
    ('order.payment_timeout_minutes', '15', 'Payment deadline in minutes after order creation'),
    ('post.rate_limit_per_hour', '5', 'Maximum posts per hour per user'),
    ('post.auto_flag_report_count', '3', 'Number of reports to auto-flag a post'),
    ('ticket.sla_response_hours', '4', 'Business hours for initial ticket response'),
    ('ticket.sla_resolution_days', '3', 'Calendar days for ticket resolution'),
    ('archive.months_threshold', '24', 'Months before data is eligible for archiving')
ON CONFLICT (key) DO NOTHING;
