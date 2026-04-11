-- Add missing id column to archive.audit_logs and add lookup hash columns
-- for pseudonymized aggregate reporting across archive tables.

-- archive.audit_logs was missing a primary key / id column
ALTER TABLE archive.audit_logs ADD COLUMN IF NOT EXISTS id UUID;

-- Add user_id_hash columns for pseudonymized aggregate queries.
-- These allow grouping by user across archived data without exposing
-- the raw user_id, enabling safe reporting on anonymised datasets.
ALTER TABLE archive.orders ADD COLUMN IF NOT EXISTS user_id_hash VARCHAR(64);
ALTER TABLE archive.tickets ADD COLUMN IF NOT EXISTS created_by_hash VARCHAR(64);

-- Populate hash columns for any rows already archived (idempotent)
UPDATE archive.orders SET user_id_hash = encode(sha256(user_id::text::bytea), 'hex')
  WHERE user_id_hash IS NULL;
UPDATE archive.tickets SET created_by_hash = encode(sha256(created_by::text::bytea), 'hex')
  WHERE created_by_hash IS NULL;
