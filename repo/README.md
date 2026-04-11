# CampusRec - Seat & Commerce Operations Platform

A production-ready, full-stack platform for campus recreation management. CampusRec handles session scheduling, seat reservations, e-commerce (products, cart, orders, shipping), ticketing with SLA enforcement, community posts with moderation, KPI dashboards, and administrative operations including backup/restore and data archiving.

## Architecture

| Layer | Technology |
|-------|-----------|
| Backend API | Go 1.23, Gin v1.10 |
| Database | PostgreSQL 16 |
| Frontend | Server-side Go templates + Tailwind CSS + Chart.js, served via Nginx |
| Orchestration | Docker Compose |

### Services

| Service | Description | Port |
|---------|-------------|------|
| `init-secrets` | Auto-generates cryptographic secrets on first startup | - |
| `db` | PostgreSQL 16 database | 5432 |
| `backend` | Go API server (migrations, admin bootstrap, REST API) | 8080 |
| `worker` | Background job scheduler (11 scheduled jobs) | - |
| `frontend` | Nginx reverse proxy serving HTML pages | 3000 |
| `test-runner` | Go test container (test profile only) | - |

### Directory Structure

```
repo/
  go.mod                     # Go module (campusrec)
  go.sum                     # Go dependency checksums
  cmd/
    server/main.go           # API server entrypoint
    worker/main.go           # Background worker entrypoint
  internal/
    config/                  # Configuration loading from env + secrets
    database/                # Connection, migrations, migration SQL files
    handlers/                # HTTP request handlers (API + page rendering)
    middleware/               # Auth, RBAC, CORS, rate limiting
    models/                  # Domain structs
    repository/              # Database access layer
    services/                # Business logic layer
    templates/               # Server-side HTML templates (layouts + pages)
    worker/                  # Scheduler and job infrastructure
  unit_tests/                # Unit tests (no DB required)
  API_tests/                 # Integration tests (run against live backend)
  backend/
    Dockerfile               # Production multi-stage build
    Dockerfile.test          # Test runner image
    entrypoint.sh            # Container entrypoint (loads secrets, runs migrations)
  frontend/
    nginx.conf               # Reverse proxy config
    static/                  # Static assets
    Dockerfile               # Nginx image
  scripts/
    init-secrets/            # Secret generation script + Dockerfile
  docker-compose.yml         # Full orchestration
  run_tests.sh               # Test runner script
  PLAN.md                    # Architecture and design plan
```

## Quick Start

### Prerequisites

- Docker Engine 20.10+
- Docker Compose v2

### Start the Application

```bash
docker compose up
```

This single command:

1. **init-secrets** generates cryptographic secrets (`db_password`, `jwt_secret`, `admin_bootstrap_password`, `wechat_merchant_key`, `backup_encryption_key`) into a shared Docker volume
2. **db** starts PostgreSQL and waits until healthy
3. **backend** loads secrets from the volume, runs all database migrations, bootstraps the admin user, and starts the API server
4. **worker** starts 11 background jobs (waitlist promotion, order closing, no-show detection, SLA checking, job processing, archiving, backups, and more)
5. **frontend** starts Nginx, proxying to the backend

### Access the Application

| URL | Description |
|-----|-------------|
| http://localhost:3000 | Web UI (Nginx proxy) |
| http://localhost:8080/api/health | Backend health check (direct) |

### Admin Credentials

The admin account is auto-created on first startup:

- **Username:** `admin`
- **Password:** Auto-generated. Retrieve it with:

```bash
docker compose exec backend cat /run/secrets/admin_bootstrap_password
```

### Verification Steps

1. **Confirm all services are running:**
   ```bash
   docker compose ps
   ```
   All services should show `Up` (except `init-secrets` which exits after completion).

2. **Check backend health:**
   ```bash
   curl http://localhost:8080/api/health
   ```
   Expected response:
   ```json
   {"code":200,"msg":"OK","data":{"status":"healthy","database":"connected","version":"1.0.0"}}
   ```

3. **Open the web UI:**
   Navigate to http://localhost:3000. You should see the login page.

4. **Log in as admin:**
   Use username `admin` and the password from the secrets volume. You will see the admin dashboard with KPI overview cards and quick action links.

5. **Run the test suite:**
   ```bash
   bash run_tests.sh
   ```
   The script builds all containers, starts services, runs unit tests and API integration tests, prints a PASS/FAIL summary, and cleans up.

## Authentication & Authorization

- **JWT-based authentication** using HMAC-SHA256 with 24-hour token expiry
- Tokens are delivered via `session_token` HTTP-only cookie and accepted via `Authorization: Bearer` header
- **Role hierarchy:** Member < Staff < Moderator < Admin

| Role | Access |
|------|--------|
| Member | Own profile, catalog, registrations, cart, orders, addresses, posts, tickets |
| Staff | Member access + shipping management, check-in operations, import/export |
| Moderator | Staff access + post moderation queue |
| Admin | Full access including user management, session/facility management, KPI, config, backup/restore, archiving |

## API Reference

All API responses follow the format `{"code": <int>, "msg": "<string>", "data": <object|array|null>}`.

### Public

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check |
| POST | `/api/auth/login` | Login (returns JWT cookie) |

### Authenticated (All Roles)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/logout` | Clear session |
| GET | `/api/auth/me` | Current user info |
| POST | `/api/auth/change-password` | Change password |
| GET | `/api/catalog` | Search catalog (sessions + products) |
| GET | `/api/sessions` | List sessions |
| GET | `/api/sessions/:id` | Session detail |
| GET | `/api/products` | List products |
| GET | `/api/products/:id` | Product detail |
| GET | `/api/addresses` | List addresses |
| POST | `/api/addresses` | Create address |
| PUT | `/api/addresses/:id` | Update address |
| DELETE | `/api/addresses/:id` | Delete address |
| PUT | `/api/addresses/:id/default` | Set default address |
| POST | `/api/registrations` | Register for session |
| GET | `/api/registrations` | List registrations |
| PUT | `/api/registrations/:id/confirm` | Confirm registration |
| PUT | `/api/registrations/:id/cancel` | Cancel registration |
| GET | `/api/cart` | View cart |
| POST | `/api/cart` | Add to cart |
| PUT | `/api/cart/:id` | Update cart item quantity |
| DELETE | `/api/cart/:id` | Remove cart item |
| POST | `/api/orders` | Create order from cart |
| GET | `/api/orders` | List orders |
| GET | `/api/orders/:id` | Order detail |
| PUT | `/api/orders/:id/cancel` | Cancel order |
| POST | `/api/orders/:id/complete` | Complete order (confirm delivery) |
| POST | `/api/posts` | Create post |
| GET | `/api/posts` | List posts |
| POST | `/api/posts/:id/report` | Report post |
| POST | `/api/tickets` | Create support ticket |
| GET | `/api/tickets` | List tickets (own for members, all for staff) |
| GET | `/api/tickets/:id` | Ticket detail |
| POST | `/api/tickets/:id/comments` | Add comment |
| GET | `/api/waitlist/position` | Check waitlist position |
| GET | `/api/checkin/:id` | Check-in detail |
| POST | `/api/checkin/:id/break` | Start break |
| POST | `/api/checkin/:id/return` | Return from break |

### Staff / Moderator

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/staff/shipping` | List shipments |
| PUT | `/api/staff/shipping/:id/ship` | Mark shipped |
| PUT | `/api/staff/shipping/:id/deliver` | Mark delivered |
| PUT | `/api/staff/shipping/:id/exception` | Mark exception |
| POST | `/api/checkin` | Perform check-in |
| GET | `/api/sessions/:id/qr` | Generate session QR code |
| GET | `/api/moderation/posts` | Moderation queue |
| POST | `/api/moderation/posts/:id/decision` | Approve/reject post |
| PUT | `/api/tickets/:id/assign` | Assign ticket |
| PUT | `/api/tickets/:id/status` | Update ticket status |
| POST | `/api/import` | Import data (CSV/XLSX) |
| GET | `/api/export` | Export data (CSV/XLSX) |
| GET | `/api/jobs/:id` | Check import/export job status |

### Admin Only

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/admin/users` | List all users |
| POST | `/api/admin/users` | Create user |
| PUT | `/api/admin/users/:id/status` | Suspend/ban/activate user |
| GET | `/api/admin/facilities` | List facilities |
| POST | `/api/admin/facilities` | Create facility |
| PUT | `/api/admin/facilities/:id` | Update facility |
| POST | `/api/admin/facilities/:id/rotate-kiosk-token` | Rotate kiosk token |
| POST | `/api/admin/sessions` | Create session |
| PUT | `/api/admin/sessions/:id` | Update session |
| PUT | `/api/admin/sessions/:id/status` | Update session status |
| GET | `/api/admin/registrations` | List all registrations |
| PUT | `/api/admin/registrations/:id/approve` | Approve registration |
| PUT | `/api/admin/registrations/:id/reject` | Reject registration |
| POST | `/api/admin/orders/:id/refund` | Refund order |
| GET | `/api/admin/config` | List configuration |
| GET | `/api/admin/config-canary` | List canary config |
| GET | `/api/admin/config-audit-logs` | Config audit logs |
| PUT | `/api/admin/config/:key` | Update config value |
| POST | `/api/admin/backup` | Trigger backup |
| GET | `/api/admin/backups` | List backups |
| GET | `/api/admin/backup/restore-targets` | Available restore targets |
| POST | `/api/admin/backup/restore` | Restore from backup |
| POST | `/api/admin/archive/run` | Trigger data archiving |
| GET | `/api/admin/archive/status` | Archive status |
| GET | `/api/kpi/overview` | KPI overview |
| GET | `/api/kpi/fill-rate` | Session fill rate |
| GET | `/api/kpi/members` | Member growth |
| GET | `/api/kpi/engagement` | Engagement metrics |
| GET | `/api/kpi/coaches` | Coach metrics |
| GET | `/api/kpi/revenue` | Revenue metrics |
| GET | `/api/kpi/tickets` | Ticket metrics |

## Frontend Pages

| Route | Role | Description |
|-------|------|-------------|
| `/login` | Public | Login form |
| `/` | Auth | Redirect to dashboard |
| `/dashboard` | Auth | Role-based dashboard with KPI cards (admin) or quick actions |
| `/catalog` | Auth | Browse sessions and products with search and filters |
| `/sessions/:id` | Auth | Session detail with register/waitlist action |
| `/products/:id` | Auth | Product detail with add-to-cart |
| `/registrations` | Member | View and manage session registrations |
| `/cart` | Member | Shopping cart with quantity controls |
| `/checkout` | Member | Address selection and order placement |
| `/orders` | Member | Order history |
| `/orders/:id` | Member | Order detail with items and shipping info |
| `/addresses` | Member | Address CRUD management |
| `/posts` | Auth | Community posts with reporting |
| `/tickets` | Auth | Support tickets list |
| `/ticket/new` | Auth | Create new ticket |
| `/ticket/:id` | Auth | Ticket detail with comments and status updates |
| `/checkin/status` | Auth | Check-in status and waitlist position |
| `/staff/shipping` | Staff | Shipping management with status actions |
| `/moderation` | Moderator | Post moderation queue |
| `/admin/users` | Admin | User management |
| `/admin/sessions` | Admin | Session management |
| `/admin/kpi` | Admin | KPI dashboard with charts |
| `/admin/config` | Admin | System configuration, canary, and audit logs |
| `/admin/import-export` | Admin | Data import/export |
| `/admin/backup` | Admin | Backup, restore, and archiving |
| `/admin/tickets` | Admin | All tickets with SLA status |

## Background Worker Jobs

The worker service runs scheduled jobs using PostgreSQL advisory locks for distributed safety:

| Job | Interval | Description |
|-----|----------|-------------|
| Waitlist Promoter | 10s | Promotes waitlisted members when seats become available |
| Order Closer | 30s | Closes unpaid orders after payment deadline |
| No-Show Detector | 30s | Detects no-shows for checked-in sessions |
| Break Overrun Detector | 15s | Flags breaks exceeding allowed duration |
| Session Status Updater | 60s | Updates session statuses based on schedule |
| SLA Checker | 5 min | Checks ticket SLA response/resolution deadlines |
| Job Processor | 5s | Processes queued import/export jobs |
| Archiver | 24 hr | Archives orders/tickets older than 24 months with PII masking |
| Backup Executor | 10s | Executes pending backup requests |
| Daily Backup | 24 hr | Triggers automatic daily database backup |
| Occupancy Anomaly Detector | 60s | Detects anomalous facility occupancy levels |

## Database

- **PostgreSQL 16** with 26 migration files applied in order on startup
- Tables: `users`, `facilities`, `sessions`, `seats`, `registrations`, `waitlist`, `products`, `addresses`, `cart_items`, `orders`, `order_items`, `payments`, `invoices`, `checkins`, `shipping`, `posts`, `moderation_decisions`, `tickets`, `ticket_comments`, `audit_logs`, `config`, `jobs`, `files`, `backups`
- Archive schema: `archive.orders`, `archive.order_items`, `archive.payments`, `archive.audit_logs`, `archive.tickets`, `archive.ticket_comments`
- Advisory locks used for worker job coordination (`FOR UPDATE SKIP LOCKED`)

## Security

- All secrets are auto-generated cryptographically (`openssl rand`) and stored in a Docker volume — no `.env` files required
- Passwords hashed with bcrypt (default cost)
- JWT tokens signed with HMAC-SHA256, 24-hour expiry
- Role-based access control enforced at middleware level
- Input validation on all API endpoints
- Structured JSON error responses (no stack traces or internal details exposed)
- Rate limiting on API requests
- CORS middleware configured
- HTTP-only, Secure cookies for session tokens (Secure flag configurable via `COOKIE_SECURE` env var)
- SQL injection prevention via parameterized queries
- XSS prevention via Go template auto-escaping

## Testing

### Run All Tests

```bash
bash run_tests.sh
```

This script:
1. Cleans up any previous test environment
2. Builds all services including the test runner
3. Starts the database and waits for readiness
4. Starts the backend and worker, waits for health check
5. Runs unit tests (no database dependency)
6. Runs API integration tests against the live backend
7. Prints a PASS/FAIL summary
8. Cleans up all containers and volumes

### Unit Tests (`unit_tests/`)

Test pure business logic without database dependencies:
- JWT token generation and validation
- Password, phone, and postal code validation rules
- SLA deadline calculation (business hours, weekends)
- Pagination computation
- Order status transitions and total calculation
- Registration state machine
- Moderation decision values
- Config canary cohort distribution

### API Integration Tests (`API_tests/`)

Test real HTTP endpoints against the running backend:
- Authentication flow (login, logout, me, change password, RBAC)
- Admin operations (users, facilities, config, backups, archive)
- Catalog search, filtering, and pagination
- Session and product listing
- Order lifecycle and cart operations
- Ticket creation and listing
- KPI endpoints
- Shipping management
- Post creation and moderation
- Address CRUD cycle
- Authorization enforcement (401/403 for unauthorized access)

## Stopping the Application

```bash
docker compose down
```

To also remove volumes (database data, secrets, backups):

```bash
docker compose down -v
```
