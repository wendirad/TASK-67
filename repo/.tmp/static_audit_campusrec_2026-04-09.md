# CampusRec Static Delivery Acceptance & Architecture Audit

Date: 2026-04-09  
Mode: Static-only (no runtime execution)

## 1. Verdict
- Overall conclusion: **Fail**
- Primary basis: Multiple **Blocker/High** defects in core prompt-critical flows (payment lifecycle, backup/restore/archiving correctness, security authorization gap, and prompt-to-implementation mismatches).

## 2. Scope and Static Verification Boundary
- Reviewed:
  - Documentation/config/manifests: `README.md`, `docker-compose.yml`, `run_tests.sh`, `go.mod`
  - Entry points/routes/middleware: `cmd/server/main.go`, `cmd/worker/main.go`, `internal/middleware/*`
  - Core domains: orders/payments, registrations/seats/waitlist/check-in, shipping, moderation, tickets, KPI, config, backup/archive/import-export
  - Migrations/schema: `internal/database/migrations/*.sql`
  - Frontend templates and page handlers: `internal/templates/*`, `internal/handlers/pages.go`
  - Tests: `unit_tests/*`, `API_tests/*`
- Not reviewed in depth:
  - Docker image runtime behavior, real browser behavior, and live DB execution outcomes
- Intentionally not executed:
  - Project startup, tests, Docker, worker jobs, any external services
- Claims requiring manual verification:
  - Real callback handling behavior (if added externally)
  - Real scheduler execution timings and idempotency under load
  - Backup file encryption and restore/PITR operational correctness
  - UI runtime rendering behavior for all pages (static mismatches are identified)

## 3. Repository / Requirement Mapping Summary
- Prompt core goal mapped: campus recreation platform combining session registration/seat control and offline commerce with RBAC, check-in controls, moderation, KPI, import/export, backup/PITR/archive, and internal-network operation.
- Main implementation areas mapped:
  - Backend REST + worker with Gin/PostgreSQL (`cmd/server/main.go`, `cmd/worker/main.go`)
  - Domain services/repositories for major modules under `internal/services` and `internal/repository`
  - HTML frontend templates under `internal/templates`
- Major prompt-fit gaps found:
  - Frontend is Go HTML templates, not Templ
  - Payment callback/signature verification and paid transition are not implemented
  - Backup/restore/PITR execution is not implemented beyond record/validation surfaces
  - Archiving contains SQL/schema mismatches that likely break ticket archival

## 4. Section-by-section Review

### 4.1 Hard Gates

#### 4.1.1 Documentation and static verifiability
- Conclusion: **Partial Pass**
- Rationale: README provides clear startup/test instructions and project structure, but startup/testing instructions are Docker-only (not usable under this static boundary), and several documented capabilities are inconsistent with code.
- Evidence:
  - Startup/testing docs: `README.md:61`, `README.md:70`, `README.md:123`, `README.md:315`
  - Structure docs: `README.md:25`
  - Worker docs mismatch actual scheduled jobs: `README.md:284`, `cmd/worker/main.go:34`
- Manual verification note: Runtime startup instructions themselves were not executed by design.

#### 4.1.2 Material deviation from Prompt
- Conclusion: **Fail**
- Rationale: Key prompt constraints are materially deviated (Templ requirement, private-network-only operation, payment callback/signature lifecycle).
- Evidence:
  - Prompt-required Templ not used; Go templates used: `internal/handlers/pages.go:4`, `internal/handlers/pages.go:12`, `go.mod:5`
  - External CDN dependencies conflict with private-network-only operation: `internal/templates/layouts/base.html:7`, `internal/templates/layouts/base.html:8`
  - No payment callback route registered: `cmd/server/main.go:224`, `cmd/server/main.go:381`

### 4.2 Delivery Completeness

#### 4.2.1 Coverage of explicit core requirements
- Conclusion: **Fail**
- Rationale: Multiple explicit core requirements are unimplemented or broken:
  - WeChat callback signature verification absent
  - Paid transition path absent
  - Backup/restore/PITR operational flow absent
  - Ticket archival SQL mismatches
  - Excel import not supported (CSV only)
- Evidence:
  - Payment callback/signature flow absent in routes and services: `cmd/server/main.go:280`, `cmd/server/main.go:381`, `internal/repository/order.go:125`
  - Payment only created as pending; no paid/confirmed transition in order logic: `internal/repository/order.go:127`, `internal/repository/order.go:309`, `internal/repository/order.go:387`
  - Backup only record/validation level: `internal/services/backup.go:19`, `internal/services/backup.go:51`
  - Archive ticket SQL uses wrong columns (`category`, `body`): `internal/repository/backup.go:379`, `internal/repository/backup.go:402`, vs schema `internal/database/migrations/018_create_tickets.sql:4`, `internal/database/migrations/018_create_tickets.sql:35`
  - Import restricted to CSV extension only: `internal/services/import_export.go:70`
- Manual verification note: Runtime behavior could worsen/improve, but these are direct static gaps.

#### 4.2.2 End-to-end 0→1 deliverable vs partial/demo
- Conclusion: **Partial Pass**
- Rationale: Repository has full multi-module structure and many implemented flows, but critical flows remain incomplete/broken, preventing acceptance as full end-to-end delivery for the prompt.
- Evidence:
  - Full structure exists: `README.md:25`, `cmd/server/main.go:17`, `cmd/worker/main.go:17`
  - Core incomplete flow examples: `internal/services/backup.go:21`, `internal/services/backup.go:79`, `internal/repository/order.go:127`

### 4.3 Engineering and Architecture Quality

#### 4.3.1 Structure and module decomposition
- Conclusion: **Pass**
- Rationale: Clear layered decomposition (handlers/services/repositories/models/migrations/worker) suitable for project scope.
- Evidence:
  - Project layering documented and reflected in code: `README.md:34`, `cmd/server/main.go:35`, `internal/services/order.go:10`, `internal/repository/order.go:12`

#### 4.3.2 Maintainability and extensibility
- Conclusion: **Partial Pass**
- Rationale: Architecture is maintainable in shape, but several hard-coded inconsistencies and schema/query drift indicate weak change safety in key modules.
- Evidence:
  - Schema/query drift example (sessions import column mismatch): `internal/worker/jobs/job_processor.go:281`, `internal/database/migrations/003_create_sessions.sql:11`
  - KPI payment-status semantic drift: `internal/repository/kpi.go:310`, `internal/database/migrations/012_create_payments.sql:6`

### 4.4 Engineering Details and Professionalism

#### 4.4.1 Error handling, logging, validation, API design
- Conclusion: **Partial Pass**
- Rationale: Basic validation/error handling patterns are present, but key professional gaps exist (missing audit logging coverage for critical status transitions, broken SQL references, weak CORS posture).
- Evidence:
  - Validation and consistent API response helpers present: `internal/handlers/auth.go:27`, `internal/handlers/response.go:8`
  - CORS insecure/misconfigured (`*` + credentials): `internal/middleware/cors.go:12`, `internal/middleware/cors.go:15`
  - Audit log insertion focused on config updates only: `internal/services/config.go:77`, `internal/repository/config.go:74`
  - Order/payment status changes without corresponding audit write path in module: `internal/repository/order.go:309`, `internal/repository/order.go:348`, `internal/repository/order.go:387`

#### 4.4.2 Real product/service shape vs demo
- Conclusion: **Partial Pass**
- Rationale: Overall shape is product-like, but critical business paths are incomplete enough to resemble partially delivered production behavior.
- Evidence:
  - Product-like breadth: `README.md:3`, `cmd/server/main.go:240`
  - Incomplete critical paths: `internal/services/backup.go:21`, `internal/services/backup.go:79`, `internal/repository/backup.go:379`

### 4.5 Prompt Understanding and Requirement Fit

#### 4.5.1 Business goal and constraint fit
- Conclusion: **Fail**
- Rationale: Several core prompt semantics are not met, including Templ rendering requirement, offline/private-network strictness, WeChat callback signature verification, and full backup/PITR execution.
- Evidence:
  - Non-Templ rendering: `internal/handlers/pages.go:4`
  - External CDN calls: `internal/templates/layouts/base.html:7`
  - Callback/signature flow missing: `cmd/server/main.go:280`, `cmd/server/main.go:381`
  - Backup/PITR not executed: `internal/services/backup.go:19`, `internal/services/backup.go:51`

### 4.6 Aesthetics (frontend-only/full-stack)

#### 4.6.1 Visual/interaction quality fit
- Conclusion: **Cannot Confirm Statistically**
- Rationale: Static templates provide structure and responsive classes, but full visual/interaction quality requires runtime rendering and browser behavior checks.
- Evidence:
  - Shared layout and responsive classes: `internal/templates/layouts/base.html:2`, `internal/templates/layouts/base.html:68`
  - Dynamic page scripts: `internal/templates/pages/catalog.html:23`
- Manual verification note: Visual hierarchy, interactive feedback, and rendering correctness require browser verification.

## 5. Issues / Suggestions (Severity-Rated)

### Blocker

1) **Payment lifecycle cannot reach paid/confirmed state (core commerce flow broken)**
- Severity: **Blocker**
- Conclusion: **Fail**
- Evidence:
  - No callback/confirm route in API registration: `cmd/server/main.go:280`, `cmd/server/main.go:381`
  - Payment created only as pending: `internal/repository/order.go:123`, `internal/repository/order.go:127`
  - Existing transitions only close/fail/refund paths: `internal/repository/order.go:309`, `internal/repository/order.go:387`, `internal/repository/order.go:406`, `internal/repository/order.go:362`
  - Shipping requires order status `paid`: `internal/repository/shipping.go:159`
- Impact: Orders can time out/close, but no complete paid lifecycle is statically implemented; downstream shipping/refund correctness is compromised.
- Minimum actionable fix: Add explicit payment confirmation/callback endpoint, verify signature using local merchant key, atomically transition payment to `confirmed` and order to `paid`, then persist immutable audit entries.

2) **Backup/restore/PITR operational flow missing (record-only behavior)**
- Severity: **Blocker**
- Conclusion: **Fail**
- Evidence:
  - Backup trigger creates DB record only: `internal/services/backup.go:21`, `internal/repository/backup.go:20`
  - Restore endpoint validates and returns accepted without restore execution path: `internal/services/backup.go:52`, `internal/services/backup.go:79`, `internal/services/backup.go:102`
  - Worker schedules no backup execution job: `cmd/worker/main.go:34`, `cmd/worker/main.go:88`
- Impact: Prompt-required nightly encrypted backup and PITR restore capability are not materially delivered.
- Minimum actionable fix: Implement backup producer/worker execution (file creation + encryption + status completion/failure), plus real snapshot and PITR restore workflows with guarded execution and auditable state transitions.

3) **Ticket archiving SQL/schema mismatch likely breaks archive execution**
- Severity: **Blocker**
- Conclusion: **Fail**
- Evidence:
  - Archive query references `category`/`body`: `internal/repository/backup.go:379`, `internal/repository/backup.go:402`
  - Actual public/archive schema uses `type`/`content`: `internal/database/migrations/018_create_tickets.sql:4`, `internal/database/migrations/018_create_tickets.sql:35`, `internal/database/migrations/022_create_archive_schema.sql:62`, `internal/database/migrations/022_create_archive_schema.sql:88`
- Impact: Ticket archival path can fail at SQL level, undermining data-retention requirement.
- Minimum actionable fix: Align archive SQL with current schemas (`type`, `content`) and add migration-safe integration tests for archive jobs.

### High

4) **Object-level authorization gap on import/export job retrieval**
- Severity: **High**
- Conclusion: **Fail**
- Evidence:
  - Route exposes `/api/jobs/:id` to any authenticated user: `cmd/server/main.go:368`
  - Handler/service fetches job by ID only; no ownership/role guard: `internal/handlers/import_export.go:74`, `internal/services/import_export.go:212`
- Impact: Authenticated users can query arbitrary job IDs and potentially access other users’ import/export metadata/results.
- Minimum actionable fix: Enforce owner-or-admin/staff check in service/repo (e.g., compare `created_by` or payload user ID).

5) **Prompt-required frontend rendering technology mismatch (Templ not implemented)**
- Severity: **High**
- Conclusion: **Fail**
- Evidence:
  - HTML templates with `html/template`: `internal/handlers/pages.go:4`, `internal/handlers/pages.go:36`
  - No Templ dependency in module: `go.mod:5`
- Impact: Explicit prompt requirement is unmet.
- Minimum actionable fix: Replace page rendering with Templ-generated views and wire handlers to Templ components.

6) **Private-network constraint violated by external CDN dependencies**
- Severity: **High**
- Conclusion: **Fail**
- Evidence:
  - Tailwind CDN: `internal/templates/layouts/base.html:7`
  - Chart.js CDN: `internal/templates/layouts/base.html:8`
- Impact: Platform depends on public internet assets, conflicting with private-network/offline operation constraint.
- Minimum actionable fix: Vendor static assets locally and serve from internal/static paths.

7) **Import sessions likely fails due wrong DB column name**
- Severity: **High**
- Conclusion: **Fail**
- Evidence:
  - Insert uses `registration_close_before_min`: `internal/worker/jobs/job_processor.go:281`
  - Schema defines `registration_close_before_minutes`: `internal/database/migrations/003_create_sessions.sql:11`
- Impact: Session import job execution likely fails for valid data.
- Minimum actionable fix: Correct column name and add integration tests for session CSV import success path.

8) **Revenue KPI likely incorrect due invalid payment status filter**
- Severity: **High**
- Conclusion: **Fail**
- Evidence:
  - KPI queries filter `p.status = 'paid'`: `internal/repository/kpi.go:310`, `internal/repository/kpi.go:323`, `internal/repository/kpi.go:353`
  - Payments valid statuses exclude `paid`: `internal/database/migrations/012_create_payments.sql:6`
- Impact: Revenue dashboard/exports can under-report or zero-out revenue.
- Minimum actionable fix: Align KPI filters to actual status model (e.g., `confirmed`) and unify status enums across modules.

9) **Immutable audit logging for critical status transitions is incomplete**
- Severity: **High**
- Conclusion: **Fail**
- Evidence:
  - Audit write path shown for config updates: `internal/services/config.go:77`, `internal/repository/config.go:74`
  - Core order/payment status transitions occur without visible audit insertion path in module: `internal/repository/order.go:309`, `internal/repository/order.go:348`, `internal/repository/order.go:387`
- Impact: Prompt-required immutable auditability for lifecycle transitions is not met.
- Minimum actionable fix: Add append-only audit writes for order/payment/shipping/refund transitions and enforce in transactional boundaries.

### Medium

10) **Frontend/API field mismatches indicate likely broken UI behaviors**
- Severity: **Medium**
- Conclusion: **Fail**
- Evidence:
  - Catalog page sends `q`, backend expects `search`: `internal/templates/pages/catalog.html:27`, `internal/handlers/catalog.go:44`
  - Catalog template expects fields not in model (`name`, `description`, `available_seats`, `stock_quantity`): `internal/templates/pages/catalog.html:48`, `internal/models/catalog.go:8`
  - Check-in status page calls waitlist endpoint without required `session_id`: `internal/templates/pages/checkin_status.html:22`, `internal/handlers/waitlist.go:21`
- Impact: Key member pages may not reflect correct data states at runtime.
- Minimum actionable fix: Align template payload contracts with API models and enforce with frontend contract tests.

11) **Import/export requirement partial: Excel not supported**
- Severity: **Medium**
- Conclusion: **Fail**
- Evidence:
  - File validation restricts to `.csv` only: `internal/services/import_export.go:70`
- Impact: Prompt explicitly states Excel/CSV offline import-export; Excel path missing.
- Minimum actionable fix: Add `.xlsx` parsing path with equivalent strict validation and duplicate fingerprinting.

12) **CORS configuration unsafe/inconsistent with credentialed auth**
- Severity: **Medium**
- Conclusion: **Fail**
- Evidence:
  - `Access-Control-Allow-Origin: *` with credentials true: `internal/middleware/cors.go:12`, `internal/middleware/cors.go:15`
- Impact: Browser behavior is undefined/insecure; may break cookie auth and weakens security posture.
- Minimum actionable fix: Restrict allowed origins explicitly and keep credentialed mode only for trusted origins.

## 6. Security Review Summary

- Authentication entry points: **Pass**
  - Evidence: login/logout/me/change-password routes and JWT middleware: `cmd/server/main.go:230`, `cmd/server/main.go:238`, `internal/middleware/auth.go:41`
  - Reasoning: Auth checks are consistently applied to protected groups.

- Route-level authorization: **Partial Pass**
  - Evidence: role middleware on moderation/admin/shipping/KPI routes: `cmd/server/main.go:297`, `cmd/server/main.go:337`, `cmd/server/main.go:314`, `cmd/server/main.go:371`
  - Reasoning: Broad route RBAC exists, but at least one sensitive route (`/api/jobs/:id`) lacks role scoping.

- Object-level authorization: **Fail**
  - Evidence:
    - Positive examples: order ownership check `internal/services/order.go:139`; ticket ownership check `internal/services/ticket.go:107`
    - Gap: job retrieval lacks owner check `internal/services/import_export.go:212`
  - Reasoning: Missing object-level check in async job retrieval creates direct data exposure risk.

- Function-level authorization: **Partial Pass**
  - Evidence: function-level ownership checks in address/ticket/order/check-in flows: `internal/services/address.go:80`, `internal/services/ticket.go:228`, `internal/services/checkin.go:137`
  - Reasoning: Present in many flows, but inconsistent across import/export job flow.

- Tenant / user data isolation: **Partial Pass**
  - Evidence: user-scoped listing/access in orders/tickets/addresses: `internal/services/order.go:151`, `internal/services/ticket.go:90`, `internal/services/address.go:19`
  - Reasoning: User isolation mostly implemented, but job endpoint weakens isolation guarantees.

- Admin / internal / debug endpoint protection: **Pass**
  - Evidence: admin group protected by `RequireRole("admin")`: `cmd/server/main.go:337`, `cmd/server/main.go:339`
  - Reasoning: No unguarded debug/admin route found in reviewed scope.

## 7. Tests and Logging Review

- Unit tests: **Partial Pass**
  - Evidence: unit suite exists: `unit_tests/auth_test.go:1`, `unit_tests/validation_test.go:1`
  - Rationale: Tests exist but are mostly pure value-map checks; limited direct binding to production logic.

- API / integration tests: **Partial Pass**
  - Evidence: integration-tagged tests exist: `API_tests/auth_api_test.go:1`, `API_tests/order_api_test.go:1`
  - Rationale: Coverage favors happy path and unauthenticated 401; limited 403/object-level/edge-concurrency/payment-callback assertions.

- Logging categories / observability: **Partial Pass**
  - Evidence:
    - Request logger: `cmd/server/main.go:427`
    - Domain action logs via `log.Printf` across services: e.g., `internal/services/order.go:124`, `internal/services/ticket.go:83`
  - Rationale: Basic logs exist but unstructured and not strongly categorized for production troubleshooting.

- Sensitive-data leakage risk in logs / responses: **Partial Pass**
  - Evidence:
    - Login success/failure logs include usernames: `internal/services/auth.go:55`, `internal/services/auth.go:69`
    - Response payload includes JWT token body in login response: `internal/handlers/auth.go:46`
  - Rationale: No direct password logging found, but identity/security metadata is logged and token is returned in JSON in addition to cookie.

## 8. Test Coverage Assessment (Static Audit)

### 8.1 Test Overview
- Unit tests exist: yes (`unit_tests/*.go`)
- API/integration tests exist: yes (`API_tests/*.go`, integration tag)
- Framework: Go `testing` package
- Test entry points documented: yes (`run_tests.sh`, README)
- Evidence:
  - Test command docs: `README.md:315`
  - Runner commands: `run_tests.sh:60`, `run_tests.sh:66`

### 8.2 Coverage Mapping Table

| Requirement / Risk Point | Mapped Test Case(s) | Key Assertion / Fixture / Mock | Coverage Assessment | Gap | Minimum Test Addition |
|---|---|---|---|---|---|
| Auth happy path and 401 unauthenticated | `API_tests/auth_api_test.go:30`, `API_tests/auth_api_test.go:110` | Valid login and `/api/auth/me` 401 checks | basically covered | No lockout-after-5 attempts behavior test | Add integration test for 5 failed logins then 423 lockout window |
| Route RBAC for admin endpoints | `API_tests/auth_api_test.go:152` | Unauthenticated admin endpoint returns 401 | insufficient | No authenticated non-admin -> 403 test | Add member/staff token tests for `/api/admin/*` expecting 403 |
| Object-level auth (own-vs-other resources) | none found for cross-user access | n/a | missing | No tests that member cannot access others’ orders/tickets/jobs | Add cross-user fixtures and 403 assertions on `/api/orders/:id`, `/api/tickets/:id`, `/api/jobs/:id` |
| Order lifecycle incl payment confirmation/callback | `API_tests/order_api_test.go:27` only basic invalid create | Fails on empty items | missing | No callback/signature/paid transition/timebox tests | Add callback verification tests + deadline close test + successful paid transition test |
| Seat/waitlist concurrency and promotion timing | none | n/a | missing | No tests for atomic seat deduction, waitlist promotion ≤30s | Add repo/service transaction tests and worker integration tests with competing registrations |
| Backup/restore/PITR correctness | `API_tests/admin_api_test.go:115`, `API_tests/admin_api_test.go:139` | Backup list and restore token validation only | insufficient | No backup execution artifact/encryption/restore tests | Add tests asserting backup status transitions, encrypted file artifact metadata, and restore execution state |
| Import validation and processing | no direct import job content tests found | n/a | insufficient | No tests for CSV/Excel parsing, duplicate fingerprint, DB write success paths | Add integration tests for import success/failure, duplicate file 409, and schema-mapped inserts |
| KPI correctness | `API_tests/kpi_api_test.go:11` | endpoint returns 200 | insufficient | No semantic assertions on returned metrics | Add seeded-data deterministic KPI assertions (revenue/fill/churn) |
| Logging sensitive data exposure | none | n/a | cannot confirm | No log-capture assertions | Add tests capturing logs for auth/order flows and assert no secret/token/password leakage |

### 8.3 Security Coverage Audit
- Authentication: **Basically covered** (login/me/401 tests exist), but lockout path untested.
- Route authorization: **Insufficient** (mostly unauthenticated 401 checks, sparse authenticated 403 checks).
- Object-level authorization: **Missing** (no cross-user ownership-denial tests).
- Tenant/data isolation: **Missing/Insufficient** (no isolation-focused integration tests).
- Admin/internal protection: **Basically covered for unauthenticated access**, insufficient for role-downgraded authenticated users.

### 8.4 Final Coverage Judgment
- **Fail**
- Boundary explanation:
  - Covered: basic endpoint reachability, some input validation, unauthenticated rejection.
  - Uncovered major risks: object-level authorization, payment callback/signature workflow, backup/restore execution, concurrency-sensitive seat/waitlist flows, and KPI semantic correctness.
  - Result: tests could pass while severe defects remain undetected.

## 9. Final Notes
- Findings are static-only and evidence-traceable; runtime success is not claimed.
- The codebase has a strong structural foundation, but prompt-critical operational and security defects prevent acceptance.
