-- Backfill canary_cohort for existing users that have NULL.
-- Uses the first 8 hex chars of the UUID, converted to a 32-bit signed int,
-- shifted to unsigned range, then MOD 100 to get a 0-99 cohort.
UPDATE users
SET canary_cohort = MOD(
    ('x' || left(replace(id::text, '-', ''), 8))::bit(32)::int::bigint + 2147483648,
    100
)::int
WHERE canary_cohort IS NULL;
