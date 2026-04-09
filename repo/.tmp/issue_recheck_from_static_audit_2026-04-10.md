# Recheck of Prior Audit Issues (2026-04-10)

Source reviewed: `.tmp/static_audit_campusrec_2026-04-09.md`  
Method: static code review only (no runtime execution)

## Summary
- Total prior issues rechecked: 12
- Fixed: 11
- Partially fixed: 1
- Still open (not fixed): 0

## Per-Issue Status

1. Payment lifecycle cannot reach paid/confirmed state
- Status: **Fixed**
- Evidence:
  - Payment callback endpoint added: `cmd/server/main.go:297`
  - Signature verification using merchant key: `internal/services/payment.go:49`, `internal/services/payment.go:50`, `cmd/server/main.go:163`
  - Atomic payment confirm + order paid transition: `internal/repository/order.go:302`, `internal/repository/order.go:324`, `internal/repository/order.go:338`

2. Backup/restore/PITR operational flow missing
- Status: **Fixed**
- Evidence:
  - Backup execution implemented (`pg_dump`, encryption): `internal/services/backup.go:167`, `internal/services/backup.go:189`, `internal/services/backup.go:200`
  - Restore execution implemented (`pg_restore`): `internal/services/backup.go:225`, `internal/services/backup.go:265`
  - Worker backup jobs added: `cmd/worker/main.go:90`, `cmd/worker/main.go:97`, `internal/worker/jobs/backup_executor.go:17`

3. Ticket archive SQL/schema mismatch
- Status: **Fixed**
- Evidence:
  - Archive uses `type` and `content` now: `internal/repository/backup.go:414`, `internal/repository/backup.go:437`
  - Matches ticket schema: `internal/database/migrations/018_create_tickets.sql:4`, `internal/database/migrations/018_create_tickets.sql:35`

4. Object-level authorization gap on `/api/jobs/:id`
- Status: **Fixed**
- Evidence:
  - Handler passes requesting user/role: `internal/handlers/import_export.go:76`, `internal/handlers/import_export.go:79`
  - Service enforces owner-or-staff/admin: `internal/services/import_export.go:224`, `internal/services/import_export.go:235`, `internal/services/import_export.go:245`, `internal/services/import_export.go:251`

5. Templ requirement not implemented
- Status: **Fixed**
- Evidence:
  - Templ-based page handler and components: `internal/handlers/pages.go:8`, `internal/handlers/pages.go:12`, `internal/handlers/pages.go:48`
  - Templ dependency present: `go.mod:22`
  - `.templ` pages present: `internal/templates/catalog.templ:1`

6. Private-network constraint violated by frontend CDNs
- Status: **Partially Fixed**
- Evidence:
  - Runtime page references local assets (not CDN URLs): `internal/templates/base.templ:16`, `internal/templates/base.templ:17`
  - But frontend image build still downloads Chart.js from internet: `frontend/Dockerfile:14`, `frontend/Dockerfile:15`
- Note: Runtime CDN dependency is removed, but build-time outbound dependency remains.

7. Import sessions wrong DB column name
- Status: **Fixed**
- Evidence:
  - Uses `registration_close_before_minutes`: `internal/worker/jobs/job_processor.go:276`, `internal/worker/jobs/job_processor.go:282`
  - Matches sessions schema: `internal/database/migrations/003_create_sessions.sql:11`

8. KPI revenue filter uses invalid payment status
- Status: **Fixed**
- Evidence:
  - Revenue queries now use `p.status = 'confirmed'`: `internal/repository/kpi.go:310`, `internal/repository/kpi.go:323`, `internal/repository/kpi.go:353`

9. Immutable audit logging incomplete for critical transitions
- Status: **Fixed**
- Evidence:
  - Shared audit repository added: `internal/repository/audit.go:10`, `internal/repository/audit.go:21`
  - Order lifecycle audit logging: `internal/services/order.go:131`, `internal/services/order.go:198`, `internal/services/order.go:232`
  - Payment confirmation audit logging: `internal/services/payment.go:94`
  - Shipping/ticket/config audit logging: `internal/services/shipping.go:64`, `internal/services/ticket.go:87`, `internal/services/config.go:78`

10. Frontend/API field mismatches (catalog and waitlist)
- Status: **Fixed**
- Evidence:
  - Catalog uses `search` query param: `internal/templates/catalog.templ:27`
  - Catalog UI fields aligned (`title`, `subtitle`, `availability_detail`): `internal/templates/catalog.templ:45`, `internal/templates/catalog.templ:46`, `internal/templates/catalog.templ:50`
  - Waitlist endpoint supports no `session_id` (active position fallback): `internal/handlers/waitlist.go:22`, `internal/handlers/waitlist.go:38`

11. Excel import/export not supported
- Status: **Fixed**
- Evidence:
  - Import accepts `.xlsx`: `internal/services/import_export.go:74`, `internal/services/import_export.go:105`
  - Export accepts `xlsx`: `internal/services/import_export.go:196`
  - Excel dependency added: `go.mod:8`, `internal/services/import_export.go:19`

12. CORS `*` with credentials misconfiguration
- Status: **Fixed**
- Evidence:
  - CORS now uses allow-list and echoes matching origin: `internal/middleware/cors.go:12`, `internal/middleware/cors.go:21`, `internal/middleware/cors.go:23`
  - Server passes configured allowed origins: `cmd/server/main.go:200`, `internal/config/config.go:58`

## Final Recheck Verdict
- Prior issue set is **substantially remediated**.
- Only remaining item from the original list is **Issue #6 (partially fixed)** due build-time internet fetch in `frontend/Dockerfile`.
