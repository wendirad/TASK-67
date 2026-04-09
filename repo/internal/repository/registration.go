package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type RegistrationRepository struct {
	db *sql.DB
}

func NewRegistrationRepository(db *sql.DB) *RegistrationRepository {
	return &RegistrationRepository{db: db}
}

func (r *RegistrationRepository) FindByID(id string) (*models.Registration, error) {
	reg := &models.Registration{}
	err := r.db.QueryRow(`
		SELECT id, user_id, session_id, status, registered_at, canceled_at, cancel_reason,
		       created_at, updated_at
		FROM registrations WHERE id = $1
	`, id).Scan(
		&reg.ID, &reg.UserID, &reg.SessionID, &reg.Status, &reg.RegisteredAt,
		&reg.CanceledAt, &reg.CancelReason, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find registration: %w", err)
	}
	return reg, nil
}

func (r *RegistrationRepository) HasActiveRegistration(userID, sessionID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM registrations
			WHERE user_id = $1 AND session_id = $2
			AND status NOT IN ('canceled', 'rejected')
		)
	`, userID, sessionID).Scan(&exists)
	return exists, err
}

// Create inserts a new registration with status 'pending'. No seat deduction.
func (r *RegistrationRepository) Create(reg *models.Registration) error {
	return r.db.QueryRow(`
		INSERT INTO registrations (user_id, session_id, status)
		VALUES ($1, $2, 'pending')
		RETURNING id, status, created_at, updated_at
	`, reg.UserID, reg.SessionID).Scan(&reg.ID, &reg.Status, &reg.CreatedAt, &reg.UpdatedAt)
}

// Approve transitions pending → approved. No capacity change.
func (r *RegistrationRepository) Approve(id string) error {
	result, err := r.db.Exec(`
		UPDATE registrations SET status = 'approved', updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
	`, id)
	if err != nil {
		return fmt.Errorf("approve registration: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("registration not in pending state")
	}
	return nil
}

// Reject transitions pending → rejected. No capacity change.
func (r *RegistrationRepository) Reject(id, reason string) error {
	result, err := r.db.Exec(`
		UPDATE registrations SET status = 'rejected', cancel_reason = $2, updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
	`, id, reason)
	if err != nil {
		return fmt.Errorf("reject registration: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("registration not in pending state")
	}
	return nil
}

// ConfirmRegistration atomically transitions approved → registered (with seat deduction)
// or approved → waitlisted (if no seats). Returns the new status.
func (r *RegistrationRepository) ConfirmRegistration(regID, sessionID, userID string) (string, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Lock the session row
	var availableSeats int
	err = tx.QueryRow(`
		SELECT available_seats FROM sessions WHERE id = $1 FOR UPDATE
	`, sessionID).Scan(&availableSeats)
	if err != nil {
		return "", fmt.Errorf("lock session: %w", err)
	}

	// Verify registration is in approved state
	var regStatus string
	err = tx.QueryRow(`
		SELECT status FROM registrations WHERE id = $1 FOR UPDATE
	`, regID).Scan(&regStatus)
	if err != nil {
		return "", fmt.Errorf("lock registration: %w", err)
	}
	if regStatus != "approved" {
		return "", fmt.Errorf("registration not in approved state (current: %s)", regStatus)
	}

	if availableSeats > 0 {
		// Deduct capacity
		_, err = tx.Exec(`
			UPDATE sessions SET available_seats = available_seats - 1, updated_at = NOW()
			WHERE id = $1
		`, sessionID)
		if err != nil {
			return "", fmt.Errorf("deduct seats: %w", err)
		}

		// Reserve a seat (not occupied)
		_, err = tx.Exec(`
			UPDATE seats SET status = 'reserved', assigned_user_id = $1, assigned_at = NOW()
			WHERE id = (
				SELECT id FROM seats WHERE session_id = $2 AND status = 'available'
				ORDER BY seat_number LIMIT 1 FOR UPDATE SKIP LOCKED
			)
		`, userID, sessionID)
		if err != nil {
			return "", fmt.Errorf("reserve seat: %w", err)
		}

		// Transition to registered
		_, err = tx.Exec(`
			UPDATE registrations SET status = 'registered', registered_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, regID)
		if err != nil {
			return "", fmt.Errorf("update registration to registered: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("commit: %w", err)
		}
		return "registered", nil
	}

	// No seats available → waitlist
	_, err = tx.Exec(`
		UPDATE registrations SET status = 'waitlisted', updated_at = NOW()
		WHERE id = $1
	`, regID)
	if err != nil {
		return "", fmt.Errorf("update registration to waitlisted: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO waitlist (session_id, user_id, position, status)
		VALUES ($1, $2,
			(SELECT COALESCE(MAX(position), 0) + 1 FROM waitlist WHERE session_id = $1 AND status = 'waiting'),
			'waiting')
	`, sessionID, userID)
	if err != nil {
		return "", fmt.Errorf("add to waitlist: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return "waitlisted", nil
}

// CancelRegistration cancels a registration. If it was 'registered', restores the seat.
// If 'waitlisted', removes from waitlist and reorders positions.
func (r *RegistrationRepository) CancelRegistration(regID, sessionID, userID, currentStatus string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE registrations SET status = 'canceled', canceled_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, regID)
	if err != nil {
		return fmt.Errorf("cancel registration: %w", err)
	}

	if currentStatus == "registered" {
		// Release the assigned seat
		_, err = tx.Exec(`
			UPDATE seats SET status = 'released', released_at = NOW(), assigned_user_id = NULL
			WHERE session_id = $1 AND assigned_user_id = $2 AND status IN ('reserved', 'occupied', 'on_break')
		`, sessionID, userID)
		if err != nil {
			return fmt.Errorf("release seat: %w", err)
		}

		// Restore available seat count
		_, err = tx.Exec(`
			UPDATE sessions SET available_seats = available_seats + 1, updated_at = NOW()
			WHERE id = $1
		`, sessionID)
		if err != nil {
			return fmt.Errorf("restore seat count: %w", err)
		}
	}

	if currentStatus == "waitlisted" {
		// Get the position of the canceled waitlist entry
		var position int
		err = tx.QueryRow(`
			SELECT position FROM waitlist WHERE session_id = $1 AND user_id = $2 AND status = 'waiting'
		`, sessionID, userID).Scan(&position)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("get waitlist position: %w", err)
		}

		// Cancel the waitlist entry
		_, err = tx.Exec(`
			UPDATE waitlist SET status = 'canceled'
			WHERE session_id = $1 AND user_id = $2 AND status = 'waiting'
		`, sessionID, userID)
		if err != nil {
			return fmt.Errorf("cancel waitlist entry: %w", err)
		}

		// Reorder positions for remaining entries
		_, err = tx.Exec(`
			UPDATE waitlist SET position = position - 1
			WHERE session_id = $1 AND position > $2 AND status = 'waiting'
		`, sessionID, position)
		if err != nil {
			return fmt.Errorf("reorder waitlist: %w", err)
		}
	}

	return tx.Commit()
}

// ListByUser returns paginated registrations for a user with optional status filter.
func (r *RegistrationRepository) ListByUser(userID string, page, pageSize int, status string) ([]models.Registration, int, error) {
	baseQuery := `FROM registrations r JOIN sessions s ON r.session_id = s.id WHERE r.user_id = $1`
	args := []interface{}{userID}
	argIdx := 2

	if status != "" {
		baseQuery += fmt.Sprintf(` AND r.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count registrations: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT r.id, r.user_id, r.session_id, r.status, r.registered_at, r.canceled_at,
		       r.cancel_reason, r.created_at, r.updated_at, s.title
		%s ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query registrations: %w", err)
	}
	defer rows.Close()

	var regs []models.Registration
	for rows.Next() {
		var reg models.Registration
		var sessionTitle string
		if err := rows.Scan(
			&reg.ID, &reg.UserID, &reg.SessionID, &reg.Status, &reg.RegisteredAt,
			&reg.CanceledAt, &reg.CancelReason, &reg.CreatedAt, &reg.UpdatedAt,
			&sessionTitle,
		); err != nil {
			return nil, 0, fmt.Errorf("scan registration: %w", err)
		}
		reg.SessionTitle = &sessionTitle
		regs = append(regs, reg)
	}

	return regs, total, rows.Err()
}

// ListAll returns paginated registrations for admin with filters.
func (r *RegistrationRepository) ListAll(page, pageSize int, sessionID, userID, status string) ([]models.Registration, int, error) {
	baseQuery := `FROM registrations r
		JOIN sessions s ON r.session_id = s.id
		JOIN users u ON r.user_id = u.id
		WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if sessionID != "" {
		baseQuery += fmt.Sprintf(` AND r.session_id = $%d`, argIdx)
		args = append(args, sessionID)
		argIdx++
	}
	if userID != "" {
		baseQuery += fmt.Sprintf(` AND r.user_id = $%d`, argIdx)
		args = append(args, userID)
		argIdx++
	}
	if status != "" {
		baseQuery += fmt.Sprintf(` AND r.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count registrations: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT r.id, r.user_id, r.session_id, r.status, r.registered_at, r.canceled_at,
		       r.cancel_reason, r.created_at, r.updated_at, s.title, u.username, u.display_name
		%s ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query registrations: %w", err)
	}
	defer rows.Close()

	var regs []models.Registration
	for rows.Next() {
		var reg models.Registration
		var sessionTitle, username, displayName string
		if err := rows.Scan(
			&reg.ID, &reg.UserID, &reg.SessionID, &reg.Status, &reg.RegisteredAt,
			&reg.CanceledAt, &reg.CancelReason, &reg.CreatedAt, &reg.UpdatedAt,
			&sessionTitle, &username, &displayName,
		); err != nil {
			return nil, 0, fmt.Errorf("scan registration: %w", err)
		}
		reg.SessionTitle = &sessionTitle
		reg.Username = &username
		reg.DisplayName = &displayName
		regs = append(regs, reg)
	}

	return regs, total, rows.Err()
}
