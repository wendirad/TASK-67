-- Enforce audit log immutability at the database level.
-- Rows in audit_logs may only be INSERTed; UPDATEs are always blocked,
-- and DELETEs are only permitted after the row has been copied to archive.audit_logs
-- (the standard archival flow). Archive rows are fully immutable.

-- 1. Block all UPDATEs on audit_logs
CREATE OR REPLACE FUNCTION audit_logs_no_update() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs rows are immutable and cannot be updated';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION audit_logs_no_update();

-- 2. Block DELETEs on audit_logs unless the row exists in archive.audit_logs
CREATE OR REPLACE FUNCTION audit_logs_no_delete() RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM archive.audit_logs WHERE id = OLD.id) THEN
        RAISE EXCEPTION 'audit_logs rows cannot be deleted without archiving first (id=%)', OLD.id;
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_logs_no_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION audit_logs_no_delete();

-- 3. Block all UPDATEs and DELETEs on archive.audit_logs (fully immutable)
CREATE OR REPLACE FUNCTION archive_audit_logs_immutable() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'archive.audit_logs rows are immutable and cannot be modified or deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_archive_audit_logs_no_update
    BEFORE UPDATE ON archive.audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION archive_audit_logs_immutable();

CREATE TRIGGER trg_archive_audit_logs_no_delete
    BEFORE DELETE ON archive.audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION archive_audit_logs_immutable();
