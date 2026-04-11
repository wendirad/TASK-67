# API Specification

This document describes the HTTP API implemented by the backend in this repository. It is derived from the actual router registration, handlers, services, and models in the codebase.

The API is mounted under `/api` unless otherwise noted. HTML page routes are intentionally excluded here.

## Conventions

### Authentication

- Authenticated routes accept the `session_token` cookie or an `Authorization: Bearer <token>` header.
- `POST /api/auth/login` returns a JWT and sets the `session_token` cookie with a 24 hour lifetime.
- The cookie is `HttpOnly` and is also used by the browser page routes.
- The authenticated user role is one of `member`, `staff`, `moderator`, or `admin`.

### Standard response envelope

Most handlers return the same JSON envelope:

```json
{
	"code": 200,
	"msg": "OK",
	"data": {}
}
```

Error responses use:

```json
{
	"code": 400,
	"msg": "Invalid request body"
}
```

### Pagination

List endpoints that support paging use `page` and `page_size` query parameters.

- `page` defaults to `1`
- `page_size` defaults to `20`
- `page_size` is capped at `100`

Paginated responses use:

```json
{
	"items": [],
	"total": 0,
	"page": 1,
	"page_size": 20,
	"total_pages": 0
}
```

### Common status codes

- `200 OK` for successful reads and updates
- `201 Created` for newly created resources
- `202 Accepted` for async jobs, backups, restores, and archive triggers
- `400 Bad Request` for invalid input or validation failures
- `401 Unauthorized` for missing or invalid authentication
- `403 Forbidden` for role or ownership failures
- `404 Not Found` when a resource does not exist or is not visible to the caller
- `409 Conflict` for duplicate or state conflicts
- `422 Unprocessable Entity` for business rule violations

## Public endpoints

| Method | Path | Auth | Notes |
| --- | --- | --- | --- |
| GET | `/api/health` | No | Health check with database status and version. |
| POST | `/api/auth/login` | No | Returns JWT token and sets `session_token`. |
| POST | `/api/payments/callback` | No | Payment provider callback. Requires signed callback payload. |

### `GET /api/health`

Response data:

```json
{
	"status": "healthy",
	"database": "connected",
	"version": "1.0.0"
}
```

If the database is unavailable, the endpoint returns `503` with `status: unhealthy` and `database: disconnected`.

### `POST /api/auth/login`

Request body:

```json
{
	"username": "alice",
	"password": "secret"
}
```

Success response data:

```json
{
	"token": "<jwt>",
	"user": {
		"id": "...",
		"username": "alice",
		"role": "member",
		"display_name": "Alice Example"
	}
}
```

## Authenticated endpoints

| Method | Path | Auth | Role | Notes |
| --- | --- | --- | --- | --- |
| POST | `/api/auth/logout` | Yes | Any | Clears the `session_token` cookie. |
| GET | `/api/auth/me` | Yes | Any | Returns the current user profile. |
| POST | `/api/auth/change-password` | Yes | Any | Changes the authenticated user's password. |
| GET | `/api/addresses` | Yes | Any authenticated user | Lists the caller's addresses. |
| POST | `/api/addresses` | Yes | Any authenticated user | Creates an address. |
| PUT | `/api/addresses/:id` | Yes | Any authenticated user | Updates an address owned by the caller. |
| DELETE | `/api/addresses/:id` | Yes | Any authenticated user | Deletes an address owned by the caller. |
| PUT | `/api/addresses/:id/default` | Yes | Any authenticated user | Marks an address as default. |
| GET | `/api/sessions` | Yes | Any authenticated user | Lists sessions with filters. |
| GET | `/api/sessions/:id` | Yes | Any authenticated user | Returns one session. |
| GET | `/api/sessions/:id/qr` | Yes | Staff/Admin | Returns a QR payload for a session. |
| GET | `/api/products` | Yes | Any authenticated user | Lists products with filters. |
| GET | `/api/products/:id` | Yes | Any authenticated user | Returns one product. |
| GET | `/api/catalog` | Yes | Any authenticated user | Unified catalog search across sessions and products. |
| GET | `/api/cart` | Yes | Any authenticated user | Returns the current cart. |
| POST | `/api/cart` | Yes | Any authenticated user | Adds an item to the cart. |
| PUT | `/api/cart/:id` | Yes | Any authenticated user | Updates a cart item quantity. |
| DELETE | `/api/cart/:id` | Yes | Any authenticated user | Removes a cart item. |
| POST | `/api/registrations` | Yes | Any authenticated user | Creates a registration. |
| GET | `/api/registrations` | Yes | Any authenticated user | Lists the caller's registrations. |
| PUT | `/api/registrations/:id/confirm` | Yes | Any authenticated user | Confirms an approved registration. |
| PUT | `/api/registrations/:id/cancel` | Yes | Any authenticated user | Cancels a registration. |
| POST | `/api/orders` | Yes | Any authenticated user | Creates an order. |
| GET | `/api/orders` | Yes | Any authenticated user | Lists orders visible to the caller. |
| GET | `/api/orders/:id` | Yes | Any authenticated user | Returns one order if visible to the caller. |
| PUT | `/api/orders/:id/cancel` | Yes | Any authenticated user | Cancels a pending payment order. |
| POST | `/api/orders/:id/complete` | Yes | Any authenticated user | Marks an order complete after shipping. |
| POST | `/api/payments/:id/simulate-callback` | Yes | Staff/Admin | Generates and processes a synthetic payment callback. |
| POST | `/api/posts` | Yes | Any authenticated user | Creates a post. |
| GET | `/api/posts` | Yes | Any authenticated user | Lists visible posts. |
| POST | `/api/posts/:id/report` | Yes | Any authenticated user | Reports a post for moderation. |
| GET | `/api/moderation/posts` | Yes | Moderator/Admin | Lists the moderation queue. |
| POST | `/api/moderation/posts/:id/decision` | Yes | Moderator/Admin | Records a moderation decision. |
| POST | `/api/tickets` | Yes | Any authenticated user | Creates a ticket. |
| GET | `/api/tickets` | Yes | Any authenticated user | Lists tickets visible to the caller. |
| GET | `/api/tickets/:id` | Yes | Any authenticated user | Returns one ticket if visible to the caller. |
| PUT | `/api/tickets/:id/assign` | Yes | Staff/Moderator/Admin | Assigns a ticket. |
| PUT | `/api/tickets/:id/status` | Yes | Staff/Moderator/Admin | Updates ticket status. |
| POST | `/api/tickets/:id/comments` | Yes | Any | Adds a ticket comment. |
| GET | `/api/staff/shipping` | Yes | Staff/Admin | Lists shipping records. |
| PUT | `/api/staff/shipping/:id/ship` | Yes | Staff/Admin | Marks a shipment as shipped. |
| PUT | `/api/staff/shipping/:id/deliver` | Yes | Staff/Admin | Confirms delivery. |
| PUT | `/api/staff/shipping/:id/exception` | Yes | Staff/Admin | Records a delivery exception. |
| POST | `/api/checkin` | Yes | Staff/Admin | Performs a check-in. |
| GET | `/api/checkin/:id` | Yes | Any authenticated user | Returns a check-in record visible to the caller. |
| POST | `/api/checkin/:id/break` | Yes | Any authenticated user | Starts a break for a check-in. |
| POST | `/api/checkin/:id/return` | Yes | Any authenticated user | Ends a break for a check-in. |
| GET | `/api/waitlist/position` | Yes | Any authenticated user | Returns the caller's waitlist position. |
| POST | `/api/import` | Yes | Staff/Admin | Starts an import job from CSV or XLSX. |
| GET | `/api/export` | Yes | Staff/Admin | Starts an export job. |
| GET | `/api/jobs/:id` | Yes | Any | Returns a job if the caller is allowed to view it. |

### `POST /api/auth/logout`

Clears the session cookie and returns a success envelope with no `data`.

### `GET /api/auth/me`

Returns the authenticated user profile:

```json
{
	"id": "...",
	"username": "alice",
	"role": "member",
	"display_name": "Alice Example",
	"email": "alice@example.com",
	"phone": "+15555550123",
	"status": "active",
	"created_at": "2026-04-11T12:00:00Z"
}
```

### `POST /api/auth/change-password`

Request body:

```json
{
	"current_password": "old",
	"new_password": "new-secret"
}
```

The new password must satisfy the backend password validation rules.

## Authenticated user resources

### Addresses

Request body for create/update:

```json
{
	"label": "Home",
	"recipient_name": "Alice Example",
	"phone": "+15555550123",
	"address_line1": "123 Main St",
	"address_line2": "Unit 4",
	"city": "Metro City",
	"province": "CA",
	"postal_code": "94107",
	"is_default": true
}
```

Validation enforced by the handler:

- `label`, `recipient_name`, `phone`, `address_line1`, `city`, `province`, and `postal_code` are required
- `label` must be at most 100 characters
- `phone` and `postal_code` must match backend format checks

Create and update return the full address object.

### Sessions

`GET /api/sessions` supports:

- `status`
- `facility`
- `search`
- `from_date`
- `to_date`
- `page`
- `page_size`

`GET /api/sessions/:id` returns a session with fields such as `id`, `title`, `facility_id`, `facility_name`, `start_time`, `end_time`, `total_seats`, `available_seats`, `status`, and timestamps.

`GET /api/sessions/:id/qr` returns:

```json
{
	"qr_content": "...",
	"valid_until": "2026-04-11T13:00:00Z"
}
```

### Products

`GET /api/products` supports:

- `category`
- `search`
- `status`
- `min_price`
- `max_price`
- `is_shippable`
- `page`
- `page_size`

### Catalog

`GET /api/catalog` supports:

- `type` with values `all`, `session`, `product`
- `search`
- `category`
- `facility`
- `from_date`
- `to_date`
- `sort` with values `relevance`, `price_asc`, `price_desc`, `date_asc`, `date_desc`, `name_asc`
- `page`
- `page_size`

Each catalog item includes:

- `type`
- `id`
- `title`
- `subtitle`
- `availability`
- `availability_detail`
- optional `image_url`
- optional `price_cents`
- optional `start_time`

### Cart

Cart response shape:

```json
{
	"items": [],
	"total_cents": 0,
	"item_count": 0
}
```

Add item request body:

```json
{
	"product_id": "...",
	"quantity": 2
}
```

Update item request body:

```json
{
	"quantity": 3
}
```

### Registrations

Create request body:

```json
{
	"session_id": "..."
}
```

List supports `status`, `page`, and `page_size`.

Registrations return objects with fields such as `id`, `user_id`, `session_id`, `status`, `registered_at`, `canceled_at`, `cancel_reason`, and joined session/user display fields when applicable.

### Orders

Create request body:

```json
{
	"items": [
		{
			"product_id": "...",
			"quantity": 1
		}
	],
	"shipping_address_id": "...",
	"source": "cart"
}
```

List supports `status`, `page`, and `page_size`.

Order responses include fields such as `id`, `order_number`, `status`, `total_cents`, shipping address fields, payment timestamps, line items, and payment details.

### Payments

Callback request body:

```json
{
	"transaction_id": "TX123",
	"order_number": "ORD-001",
	"amount_cents": 5000,
	"status": "SUCCESS",
	"nonce_str": "abc123",
	"sign": "<hmac-sha256>"
}
```

Rules enforced by the handler and service:

- `transaction_id`, `order_number`, `nonce_str`, and `sign` are required
- `status` must be `SUCCESS`
- the signature must match the backend HMAC computation
- `amount_cents` must equal the order total

### Posts and moderation

Create post request body:

```json
{
	"title": "Community update",
	"content": "..."
}
```

Report post request body:

```json
{
	"reason": "Spam"
}
```

Moderation decision request body:

```json
{
	"action": "approve",
	"reason": "Meets community guidelines"
}
```

### Tickets

Create ticket request body:

```json
{
	"type": "support",
	"subject": "Issue with membership",
	"description": "...",
	"priority": "high",
	"related_entity_type": "order",
	"related_entity_id": "..."
}
```

`GET /api/tickets` supports:

- `status`
- `type`
- `priority`
- `assigned_to`
- `page`
- `page_size`

Assign request body:

```json
{
	"assigned_to": "user-id"
}
```

Status update request body:

```json
{
	"status": "in_progress"
}
```

Comment request body:

```json
{
	"content": "Working on it"
}
```

Ticket responses include the ticket, optional assignee and creator profile fields, SLA metadata, timestamps, and nested comments.

### Shipping and check-in

Shipping ship request body:

```json
{
	"tracking_number": "TN123",
	"carrier": "DHL"
}
```

Delivery request body:

```json
{
	"proof_type": "photo",
	"proof_data": "base64-or-url"
}
```

Exception request body:

```json
{
	"exception_notes": "Recipient unavailable"
}
```

Check-in request body:

```json
{
	"registration_id": "...",
	"kiosk_device_token": "...",
	"bluetooth_confirmed": false,
	"bluetooth_beacon_id": "..."
}
```

Check-in responses include the check-in record, including status, method, break counters, timestamps, and joined session/user display fields.

### Waitlist

`GET /api/waitlist/position` accepts an optional `session_id` query parameter.

- With `session_id`, it returns the caller's position for that session or `404` if the user is not on the waitlist.
- Without `session_id`, it returns the caller's most recent active waitlist position, or `null` data if none exists.

## Operational endpoints

| Method | Path | Auth | Role | Notes |
| --- | --- | --- | --- | --- |
| GET | `/api/jobs/:id` | Yes | Any | Staff/admin can access any job; others only their own. |
| POST | `/api/import` | Yes | Staff/Admin | Multipart form upload. |
| GET | `/api/export` | Yes | Staff/Admin | Starts an async export job. |
| GET | `/api/kpi/overview` | Yes | Admin | High-level KPI summary. |
| GET | `/api/kpi/fill-rate` | Yes | Admin | Fill rate time series. |
| GET | `/api/kpi/members` | Yes | Admin | Member growth/churn time series. |
| GET | `/api/kpi/engagement` | Yes | Admin | Engagement metrics. |
| GET | `/api/kpi/coaches` | Yes | Admin | Coach productivity metrics. |
| GET | `/api/kpi/revenue` | Yes | Admin | Revenue summary and time series. |
| GET | `/api/kpi/tickets` | Yes | Admin | Ticket metrics summary. |

### `POST /api/import`

Multipart form fields:

- `entity_type`
- `file`

Supported entity types:

- `sessions`
- `products`
- `users`
- `registrations`

Supported file types:

- `.csv`
- `.xlsx`

The handler rejects files larger than 10 MB and rejects duplicate files by content hash.

Validation failures return `400` with a `data` object describing row-level errors, valid row count, and error count.

### `GET /api/export`

Query parameters:

- `entity_type`
- `format` with values `csv` or `xlsx`
- `filters` as a JSON string

Supported entity types:

- `sessions`
- `products`
- `users`
- `orders`
- `registrations`
- `tickets`

The endpoint returns `202 Accepted` and a job identifier.

### `GET /api/jobs/:id`

Job objects include:

- `id`
- `type`
- `status`
- `payload`
- `result`
- `attempts`
- `max_attempts`
- `scheduled_at`
- `started_at`
- `completed_at`
- `created_at`

Authorization rules:

- `staff` and `admin` can view any job
- other users can only view jobs whose payload owns the matching `user_id`

## Admin endpoints

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/api/admin/users` | List users with filters. |
| POST | `/api/admin/users` | Create a user. |
| PUT | `/api/admin/users/:id/status` | Update user status. |
| GET | `/api/admin/facilities` | List facilities. |
| POST | `/api/admin/facilities` | Create a facility. |
| PUT | `/api/admin/facilities/:id` | Update a facility. |
| POST | `/api/admin/facilities/:id/rotate-kiosk-token` | Rotate kiosk token. |
| POST | `/api/admin/sessions` | Create a session. |
| PUT | `/api/admin/sessions/:id` | Update a session. |
| PUT | `/api/admin/sessions/:id/status` | Update session status. |
| GET | `/api/admin/registrations` | List registrations with filters. |
| PUT | `/api/admin/registrations/:id/approve` | Approve a registration. |
| PUT | `/api/admin/registrations/:id/reject` | Reject a registration. |
| POST | `/api/admin/orders/:id/refund` | Refund an order. |
| GET | `/api/admin/config` | List configuration entries. |
| GET | `/api/admin/config-canary` | List canary configuration entries. |
| GET | `/api/admin/config-audit-logs` | List recent audit logs. |
| PUT | `/api/admin/config/:key` | Update a configuration key. |
| POST | `/api/admin/backup` | Trigger a backup job. |
| GET | `/api/admin/backups` | List backups. |
| GET | `/api/admin/backup/restore-targets` | List restore targets. |
| POST | `/api/admin/backup/restore` | Trigger a restore job. |
| POST | `/api/admin/archive/run` | Trigger archive processing. |
| GET | `/api/admin/archive/status` | Get archive status. |

### `GET /api/admin/users`

Supports `role`, `status`, `search`, `page`, and `page_size`.

Valid roles:

- `member`
- `staff`
- `moderator`
- `admin`

Valid statuses:

- `active`
- `banned`
- `suspended`
- `inactive`

### `POST /api/admin/users`

Request body:

```json
{
	"username": "newuser",
	"password": "secret",
	"role": "member",
	"display_name": "New User",
	"email": "newuser@example.com",
	"phone": "+15555550123"
}
```

### `PUT /api/admin/users/:id/status`

Request body:

```json
{
	"status": "active"
}
```

### `GET /api/admin/facilities`

Returns all facilities as `items`.

### `POST /api/admin/facilities`

Request body:

```json
{
	"name": "Main Gym",
	"checkin_mode": "staff_qr",
	"bluetooth_beacon_id": "optional",
	"bluetooth_beacon_range_meters": 10
}
```

Valid check-in modes:

- `staff_qr`
- `staff_qr_bluetooth`

When `checkin_mode` is `staff_qr_bluetooth`, `bluetooth_beacon_id` is required.

### `PUT /api/admin/facilities/:id`

Request body accepts the same fields as create, but all are optional.

### `POST /api/admin/sessions`

Request body:

```json
{
	"title": "Morning Class",
	"description": "Optional text",
	"coach_name": "Coach A",
	"facility_id": "...",
	"start_time": "2026-04-11T09:00:00Z",
	"end_time": "2026-04-11T10:00:00Z",
	"total_seats": 20,
	"registration_close_before_minutes": 120
}
```

### `PUT /api/admin/sessions/:id`

All fields are optional. `start_time`, `end_time`, `total_seats`, and `registration_close_before_minutes` may be updated independently when allowed by the service layer.

### `PUT /api/admin/sessions/:id/status`

Request body:

```json
{
	"status": "closed"
}
```

Valid statuses:

- `open`
- `closed`
- `canceled`

### `GET /api/admin/registrations`

Supports `session_id`, `user_id`, `status`, `page`, and `page_size`.

### `PUT /api/admin/registrations/:id/reject`

Request body:

```json
{
	"reason": "Capacity reached"
}
```

### `POST /api/admin/orders/:id/refund`

No request body.

### `PUT /api/admin/config/:key`

Request body:

```json
{
	"value": "enabled",
	"canary_percentage": 20
}
```

`canary_percentage` is optional and may be omitted.

### `POST /api/admin/backup`

Creates a backup record and returns it with `202 Accepted`.

### `GET /api/admin/backups`

Returns backup records with fields such as `id`, `filename`, `size_bytes`, `encrypted`, `type`, `status`, `wal_start_lsn`, `started_at`, `completed_at`, and `created_at`.

### `GET /api/admin/backup/restore-targets`

Returns restore window data:

- `earliest_snapshot`
- `earliest_pitr`
- `latest_pitr`
- `base_backups`

### `POST /api/admin/backup/restore`

Request body:

```json
{
	"restore_type": "snapshot",
	"backup_id": "...",
	"target_time": "2026-04-11T10:30:00Z",
	"confirmation_token": "RESTORE"
}
```

Rules enforced by the service:

- `confirmation_token` must literally be `RESTORE`
- `restore_type` must be `snapshot` or `point_in_time`
- `snapshot` requires `backup_id`
- `point_in_time` requires `target_time`

### `POST /api/admin/archive/run`

Triggers archive processing for orders and tickets and returns archive status with `202 Accepted`.

### `GET /api/admin/archive/status`

Returns the archive status object.

## KPI responses

### `GET /api/kpi/overview`

Query parameters:

- `from_date`
- `to_date`
- `facility`

Response fields:

- `fill_rate`
- `engagement`
- `revenue`
- `ticket_metrics`
- `total_members`
- `active_members`

### `GET /api/kpi/fill-rate`

Query parameters:

- `from_date`
- `to_date`
- `facility`
- `granularity` with values `daily`, `weekly`, or `monthly`

Each point includes `period`, `fill_rate`, and `session_count`.

### `GET /api/kpi/members`

Query parameters:

- `from_date`
- `to_date`
- `granularity` with values `daily`, `weekly`, or `monthly`

Each point includes `period`, `growth`, `churn`, and `net_gain`.

### `GET /api/kpi/engagement`

Returns:

- `active_members`
- `total_check_ins`
- `avg_sessions_per_member`
- `total_orders`

### `GET /api/kpi/coaches`

Each row includes:

- `coach_name`
- `session_count`
- `avg_fill_rate`

### `GET /api/kpi/revenue`

Query parameters:

- `from_date`
- `to_date`
- `granularity` with values `daily`, `weekly`, or `monthly`

Response fields:

- `total_cents`
- `time_series`
- `by_category`

### `GET /api/kpi/tickets`

Returns:

- `open_count`
- `in_progress_count`
- `resolved_count`
- `closed_count`
- `avg_resolution_hours`
- `sla_response_compliance_pct`
- `sla_resolution_compliance_pct`
- `total_tickets`

## Resource summary

The following model shapes are used across responses:

- `User`: `id`, `username`, `role`, `display_name`, `email`, `phone`, `status`, timestamps
- `Address`: `id`, `label`, `recipient_name`, `phone`, `address_line1`, `address_line2`, `city`, `province`, `postal_code`, `is_default`, `created_at`
- `Session`: `id`, `title`, `description`, `coach_name`, `facility_id`, `facility_name`, `start_time`, `end_time`, `total_seats`, `available_seats`, `registration_close_before_minutes`, `status`, timestamps
- `Product`: `id`, `name`, `description`, `category`, `price_cents`, `stock_quantity`, `is_shippable`, `image_url`, `status`, `availability`, timestamps
- `Registration`: `id`, `user_id`, `session_id`, `status`, `registered_at`, `canceled_at`, `cancel_reason`, timestamps, joined user/session fields
- `Order`: `id`, `order_number`, `status`, `total_cents`, shipping address fields, payment timestamps, items, payment details, timestamps
- `Ticket`: `id`, `ticket_number`, `type`, `subject`, `description`, `status`, `priority`, `assigned_to`, SLA metadata, nested comments, timestamps
- `CheckIn`: `id`, `registration_id`, `user_id`, `session_id`, `seat_id`, `seat_number`, `status`, `method`, `confirmed_by`, break counters, timestamps, joined session/user fields
- `Job`: `id`, `type`, `status`, `payload`, `result`, retry counters, schedule and completion timestamps
- `Backup`: `id`, `filename`, `size_bytes`, `encrypted`, `type`, `status`, `wal_start_lsn`, `started_at`, `completed_at`, `created_at`
- `ConfigEntry`: `id`, `key`, `value`, `description`, `canary_percentage`, `updated_by`, `updated_at`, `created_at`

## Notes

- `POST /api/import`, `GET /api/export`, `GET /api/jobs/:id`, and the backup endpoints are asynchronous or operational in nature and often return `202 Accepted`.
- Some endpoints perform additional ownership or role checks in the service layer after authentication.
- The API uses JSON request and response bodies except for multipart import uploads.
