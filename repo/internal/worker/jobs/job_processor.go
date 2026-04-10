package jobs

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"campusrec/internal/models"

	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

const JobProcessorLockID int64 = 106

// ImportPayload mirrors the service's ImportPayload.
type ImportPayload struct {
	EntityType string     `json:"entity_type"`
	Filename   string     `json:"filename"`
	FileID     string     `json:"file_id"`
	Rows       [][]string `json:"rows"`
	Headers    []string   `json:"headers"`
	UserID     string     `json:"user_id"`
}

// ExportPayload mirrors the service's ExportPayload.
type ExportPayload struct {
	EntityType string `json:"entity_type"`
	Format     string `json:"format"`
	Filters    string `json:"filters"`
	UserID     string `json:"user_id"`
}

// JobProcessor picks up pending jobs and processes them.
// Job pickup uses a CTE with FOR UPDATE SKIP LOCKED to atomically claim jobs,
// preventing duplicate execution under concurrent workers.
func JobProcessor(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Atomically claim pending jobs: SELECT + UPDATE in a single CTE.
		// FOR UPDATE SKIP LOCKED ensures no two workers claim the same job,
		// even if advisory locks fail or multiple worker instances start.
		rows, err := db.QueryContext(ctx, `
			WITH picked AS (
				SELECT id FROM jobs
				WHERE status = 'pending' AND scheduled_at <= NOW()
				ORDER BY created_at
				LIMIT 5
				FOR UPDATE SKIP LOCKED
			)
			UPDATE jobs SET status = 'processing', started_at = NOW(), attempts = attempts + 1
			FROM picked WHERE jobs.id = picked.id
			RETURNING jobs.id, jobs.type, jobs.payload
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		type jobEntry struct {
			ID      string
			Type    string
			Payload *string
		}
		var claimed []jobEntry
		for rows.Next() {
			var j jobEntry
			if err := rows.Scan(&j.ID, &j.Type, &j.Payload); err != nil {
				return err
			}
			claimed = append(claimed, j)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		for _, j := range claimed {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			var result string
			var procErr error

			if j.Payload == nil {
				procErr = fmt.Errorf("job has no payload")
			} else if isImportJob(j.Type) {
				result, procErr = processImport(ctx, db, *j.Payload)
			} else if isExportJob(j.Type) {
				result, procErr = processExport(ctx, db, *j.Payload)
			} else {
				procErr = fmt.Errorf("unknown job type: %s", j.Type)
			}

			if procErr != nil {
				log.Printf("Job processor: job %s failed: %v", j.ID, procErr)
				db.ExecContext(ctx, `
					UPDATE jobs SET
					    status = CASE WHEN attempts < max_attempts THEN 'pending' ELSE 'failed' END,
					    result = $2,
					    scheduled_at = CASE WHEN attempts < max_attempts THEN NOW() + INTERVAL '30 seconds' * attempts ELSE scheduled_at END
					WHERE id = $1
				`, j.ID, procErr.Error())
			} else {
				db.ExecContext(ctx, `
					UPDATE jobs SET status = 'completed', result = $2, completed_at = NOW()
					WHERE id = $1
				`, j.ID, result)
				log.Printf("Job processor: job %s completed", j.ID)
			}
		}

		return nil
	}
}

func isImportJob(t string) bool {
	return t == "import_sessions" || t == "import_products" || t == "import_users" || t == "import_registrations"
}

func isExportJob(t string) bool {
	return t == "export_sessions" || t == "export_products" || t == "export_users" ||
		t == "export_orders" || t == "export_registrations" || t == "export_tickets"
}

func processImport(ctx context.Context, db *sql.DB, payloadStr string) (string, error) {
	var payload ImportPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return "", fmt.Errorf("parse import payload: %w", err)
	}

	// Build field map from headers
	headerIdx := make(map[string]int)
	for i, h := range payload.Headers {
		headerIdx[h] = i
	}

	imported := 0
	skipped := 0

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, row := range payload.Rows {
		fields := make(map[string]string)
		for i, h := range payload.Headers {
			if i < len(row) {
				fields[h] = row[i]
			}
		}

		var importErr error
		var wasSkipped bool

		switch payload.EntityType {
		case "products":
			wasSkipped, importErr = importProduct(ctx, tx, fields)
		case "users":
			wasSkipped, importErr = importUser(ctx, tx, fields)
		case "sessions":
			wasSkipped, importErr = importSession(ctx, tx, fields)
		case "registrations":
			wasSkipped, importErr = importRegistration(ctx, tx, fields)
		}

		if importErr != nil {
			return "", fmt.Errorf("import row: %w", importErr)
		}
		if wasSkipped {
			skipped++
		} else {
			imported++
		}
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	result := fmt.Sprintf(`{"imported":%d,"skipped":%d,"total":%d}`, imported, skipped, len(payload.Rows))
	return result, nil
}

func importProduct(ctx context.Context, tx *sql.Tx, fields map[string]string) (bool, error) {
	// Check duplicate by name + category
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM products WHERE name = $1 AND category = $2)
	`, fields["name"], fields["category"]).Scan(&exists)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil // skip duplicate
	}

	priceCents, _ := strconv.Atoi(fields["price_cents"])
	stockQty, _ := strconv.Atoi(fields["stock_quantity"])
	isShippable := fields["is_shippable"] == "true" || fields["is_shippable"] == "1"

	_, err = tx.ExecContext(ctx, `
		INSERT INTO products (name, description, category, price_cents, stock_quantity, is_shippable, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'active')
	`, fields["name"], fields["description"], fields["category"], priceCents, stockQty, isShippable)
	return false, err
}

func importUser(ctx context.Context, tx *sql.Tx, fields map[string]string) (bool, error) {
	// Check duplicate by username
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)
	`, fields["username"]).Scan(&exists)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(fields["password"]), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("hash password: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, display_name, status)
		VALUES ($1, $2, $3, $4, 'active')
	`, fields["username"], string(hashedPassword), fields["role"], fields["display_name"])
	return false, err
}

func importSession(ctx context.Context, tx *sql.Tx, fields map[string]string) (bool, error) {
	// Look up facility by name
	var facilityID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM facilities WHERE name = $1`, fields["facility_name"]).Scan(&facilityID)
	if err != nil {
		return false, fmt.Errorf("facility '%s' not found", fields["facility_name"])
	}

	startTime, err := time.Parse(time.RFC3339, fields["start_time"])
	if err != nil {
		startTime, err = time.Parse("2006-01-02 15:04:05", fields["start_time"])
		if err != nil {
			return false, fmt.Errorf("invalid start_time format")
		}
	}

	// Check duplicate
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM sessions WHERE title = $1 AND start_time = $2 AND facility_id = $3)
	`, fields["title"], startTime, facilityID).Scan(&exists)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	endTime, err := time.Parse(time.RFC3339, fields["end_time"])
	if err != nil {
		endTime, err = time.Parse("2006-01-02 15:04:05", fields["end_time"])
		if err != nil {
			return false, fmt.Errorf("invalid end_time format")
		}
	}

	totalSeats, _ := strconv.Atoi(fields["total_seats"])
	regCloseMin := lookupRegCloseDefault(ctx, tx)
	if fields["registration_close_before_minutes"] != "" {
		regCloseMin, _ = strconv.Atoi(fields["registration_close_before_minutes"])
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (facility_id, title, description, coach_name, start_time, end_time,
		    total_seats, available_seats, registration_close_before_minutes, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8, 'open')
	`, facilityID, fields["title"], fields["description"], fields["coach_name"],
		startTime, endTime, totalSeats, regCloseMin)
	return false, err
}

func importRegistration(ctx context.Context, tx *sql.Tx, fields map[string]string) (bool, error) {
	// Look up user by username
	var userID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE username = $1`, fields["username"]).Scan(&userID)
	if err != nil {
		return false, fmt.Errorf("user '%s' not found", fields["username"])
	}

	// Look up session by title (use most recent open one)
	var sessionID string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM sessions WHERE title = $1 AND status = 'open' ORDER BY start_time DESC LIMIT 1
	`, fields["session_title"]).Scan(&sessionID)
	if err != nil {
		return false, fmt.Errorf("open session '%s' not found", fields["session_title"])
	}

	// Check duplicate
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM registrations WHERE user_id = $1 AND session_id = $2 AND status NOT IN ('canceled', 'rejected'))
	`, userID, sessionID).Scan(&exists)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO registrations (user_id, session_id, status)
		VALUES ($1, $2, 'pending')
	`, userID, sessionID)
	return false, err
}

func processExport(ctx context.Context, db *sql.DB, payloadStr string) (string, error) {
	var payload ExportPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return "", fmt.Errorf("parse export payload: %w", err)
	}

	filters, err := models.ParseExportFilters(payload.Filters)
	if err != nil {
		return "", fmt.Errorf("parse filters: %w", err)
	}

	var headers []string
	var rows [][]string

	switch payload.EntityType {
	case "users":
		headers = []string{"id", "username", "role", "display_name", "status", "created_at"}
		where, args := buildUserFilters(filters)
		rows, err = queryExport(ctx, db, `
			SELECT id::text, username, role, display_name, status, created_at::text FROM users`+where+` ORDER BY created_at
		`, args...)
	case "products":
		headers = []string{"id", "name", "category", "price_cents", "stock_quantity", "is_shippable", "status"}
		where, args := buildProductFilters(filters)
		rows, err = queryExport(ctx, db, `
			SELECT id::text, name, category, price_cents::text, stock_quantity::text, is_shippable::text, status FROM products`+where+` ORDER BY name
		`, args...)
	case "sessions":
		headers = []string{"id", "title", "facility", "coach_name", "start_time", "end_time", "total_seats", "status"}
		where, args := buildSessionFilters(filters)
		rows, err = queryExport(ctx, db, `
			SELECT s.id::text, s.title, f.name, COALESCE(s.coach_name, ''), s.start_time::text, s.end_time::text, s.total_seats::text, s.status
			FROM sessions s JOIN facilities f ON f.id = s.facility_id`+where+` ORDER BY s.start_time
		`, args...)
	case "orders":
		headers = []string{"id", "order_number", "user", "total_cents", "status", "created_at"}
		where, args := buildOrderFilters(filters)
		rows, err = queryExport(ctx, db, `
			SELECT o.id::text, o.order_number, u.username, o.total_cents::text, o.status, o.created_at::text
			FROM orders o JOIN users u ON u.id = o.user_id`+where+` ORDER BY o.created_at
		`, args...)
	case "registrations":
		headers = []string{"id", "username", "session_title", "status", "created_at"}
		where, args := buildRegistrationFilters(filters)
		rows, err = queryExport(ctx, db, `
			SELECT r.id::text, u.username, s.title, r.status, r.created_at::text
			FROM registrations r JOIN users u ON u.id = r.user_id JOIN sessions s ON s.id = r.session_id`+where+` ORDER BY r.created_at
		`, args...)
	case "tickets":
		headers = []string{"ticket_number", "type", "subject", "status", "priority", "created_at"}
		where, args := buildTicketFilters(filters)
		rows, err = queryExport(ctx, db, `
			SELECT ticket_number, type, subject, status, priority, created_at::text FROM tickets`+where+` ORDER BY created_at
		`, args...)
	default:
		return "", fmt.Errorf("unsupported export entity: %s", payload.EntityType)
	}

	if err != nil {
		return "", fmt.Errorf("query export data: %w", err)
	}

	var buf bytes.Buffer

	if payload.Format == "xlsx" {
		// Generate Excel file in memory
		f := excelize.NewFile()
		sheetName := "Sheet1"
		for col, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(col+1, 1)
			f.SetCellValue(sheetName, cell, h)
		}
		for rowIdx, row := range rows {
			for col, val := range row {
				cell, _ := excelize.CoordinatesToCellName(col+1, rowIdx+2)
				f.SetCellValue(sheetName, cell, val)
			}
		}
		if _, err := f.WriteTo(&buf); err != nil {
			return "", fmt.Errorf("generate xlsx: %w", err)
		}

		resultJSON, _ := json.Marshal(map[string]interface{}{
			"row_count":  len(rows),
			"format":     "xlsx",
			"xlsx_data":  buf.String(),
		})
		return string(resultJSON), nil
	}

	// Generate CSV in memory
	writer := csv.NewWriter(&buf)
	writer.Write(headers)
	for _, row := range rows {
		writer.Write(row)
	}
	writer.Flush()

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"row_count": len(rows),
		"format":    "csv",
		"csv_data":  buf.String(),
	})

	return string(resultJSON), nil
}

func queryExport(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([][]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var result [][]string

	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		row := make([]string, len(cols))
		for i, v := range vals {
			if v == nil {
				row[i] = ""
			} else {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// filterBuilder accumulates WHERE conditions and parameterized args.
type filterBuilder struct {
	conditions []string
	args       []interface{}
	argIdx     int
}

func newFilterBuilder() *filterBuilder {
	return &filterBuilder{argIdx: 1}
}

func (fb *filterBuilder) add(condition string, value interface{}) {
	fb.conditions = append(fb.conditions, fmt.Sprintf(condition, fb.argIdx))
	fb.args = append(fb.args, value)
	fb.argIdx++
}

func (fb *filterBuilder) build() (string, []interface{}) {
	if len(fb.conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(fb.conditions, " AND "), fb.args
}

func addDateFilters(fb *filterBuilder, filters *models.ExportFilters, dateColumn string) {
	if filters.DateFrom != "" {
		fb.add(dateColumn+" >= $%d", filters.DateFrom)
	}
	if filters.DateTo != "" {
		fb.add(dateColumn+" < ($%d::date + INTERVAL '1 day')", filters.DateTo)
	}
}

func buildUserFilters(filters *models.ExportFilters) (string, []interface{}) {
	fb := newFilterBuilder()
	if filters.Status != "" {
		fb.add("status = $%d", filters.Status)
	}
	if filters.Role != "" {
		fb.add("role = $%d", filters.Role)
	}
	addDateFilters(fb, filters, "created_at")
	return fb.build()
}

func buildProductFilters(filters *models.ExportFilters) (string, []interface{}) {
	fb := newFilterBuilder()
	if filters.Status != "" {
		fb.add("status = $%d", filters.Status)
	}
	if filters.Category != "" {
		fb.add("category = $%d", filters.Category)
	}
	addDateFilters(fb, filters, "created_at")
	return fb.build()
}

func buildSessionFilters(filters *models.ExportFilters) (string, []interface{}) {
	fb := newFilterBuilder()
	if filters.Status != "" {
		fb.add("s.status = $%d", filters.Status)
	}
	addDateFilters(fb, filters, "s.start_time")
	return fb.build()
}

func buildOrderFilters(filters *models.ExportFilters) (string, []interface{}) {
	fb := newFilterBuilder()
	if filters.Status != "" {
		fb.add("o.status = $%d", filters.Status)
	}
	addDateFilters(fb, filters, "o.created_at")
	return fb.build()
}

func buildRegistrationFilters(filters *models.ExportFilters) (string, []interface{}) {
	fb := newFilterBuilder()
	if filters.Status != "" {
		fb.add("r.status = $%d", filters.Status)
	}
	addDateFilters(fb, filters, "r.created_at")
	return fb.build()
}

func buildTicketFilters(filters *models.ExportFilters) (string, []interface{}) {
	fb := newFilterBuilder()
	if filters.Status != "" {
		fb.add("status = $%d", filters.Status)
	}
	if filters.Type != "" {
		fb.add("type = $%d", filters.Type)
	}
	if filters.Priority != "" {
		fb.add("priority = $%d", filters.Priority)
	}
	addDateFilters(fb, filters, "created_at")
	return fb.build()
}

func lookupRegCloseDefault(ctx context.Context, tx *sql.Tx) int {
	var val string
	err := tx.QueryRowContext(ctx,
		`SELECT value FROM config_entries WHERE key = 'session.reg_close_default_minutes'`,
	).Scan(&val)
	if err != nil {
		return 120
	}
	v, err := strconv.Atoi(val)
	if err != nil || v < 0 {
		return 120
	}
	return v
}
