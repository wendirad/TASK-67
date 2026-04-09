-- Allow re-registration after cancellation/rejection by making the unique index partial
DROP INDEX idx_reg_user_session;
CREATE UNIQUE INDEX idx_reg_user_session ON registrations (user_id, session_id)
WHERE status NOT IN ('canceled', 'rejected');
