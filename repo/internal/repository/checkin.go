package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type CheckInRepository struct {
	db *sql.DB
}

func NewCheckInRepository(db *sql.DB) *CheckInRepository {
	return &CheckInRepository{db: db}
}

// PerformCheckIn atomically creates a check-in record and transitions the seat from reserved to occupied.
func (r *CheckInRepository) PerformCheckIn(registrationID, userID, sessionID, confirmedBy, method string) (*models.CheckIn, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Find the reserved seat for this user/session
	var seatID string
	var seatNumber int
	err = tx.QueryRow(`
		SELECT id, seat_number FROM seats
		WHERE session_id = $1 AND assigned_user_id = $2 AND status = 'reserved'
		FOR UPDATE
	`, sessionID, userID).Scan(&seatID, &seatNumber)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no reserved seat found for this registration")
	}
	if err != nil {
		return nil, fmt.Errorf("find seat: %w", err)
	}

	// Transition seat: reserved → occupied
	_, err = tx.Exec(`
		UPDATE seats SET status = 'occupied' WHERE id = $1
	`, seatID)
	if err != nil {
		return nil, fmt.Errorf("occupy seat: %w", err)
	}

	// Create check-in record
	ci := &models.CheckIn{}
	err = tx.QueryRow(`
		INSERT INTO check_ins (registration_id, user_id, session_id, seat_id, confirmed_by, method, status, checked_in_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', NOW())
		RETURNING id, registration_id, user_id, session_id, seat_id, status, method,
		          confirmed_by, break_count, total_break_minutes, last_break_start,
		          checked_in_at, checked_out_at, created_at, updated_at
	`, registrationID, userID, sessionID, seatID, confirmedBy, method,
	).Scan(
		&ci.ID, &ci.RegistrationID, &ci.UserID, &ci.SessionID, &ci.SeatID,
		&ci.Status, &ci.Method, &ci.ConfirmedBy, &ci.BreakCount, &ci.TotalBreakMinutes,
		&ci.LastBreakStart, &ci.CheckedInAt, &ci.CheckedOutAt, &ci.CreatedAt, &ci.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert check_in: %w", err)
	}
	ci.SeatNumber = seatNumber

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return ci, nil
}

// FindByID returns a check-in by ID.
func (r *CheckInRepository) FindByID(id string) (*models.CheckIn, error) {
	ci := &models.CheckIn{}
	err := r.db.QueryRow(`
		SELECT ci.id, ci.registration_id, ci.user_id, ci.session_id, ci.seat_id,
		       ci.status, ci.method, ci.confirmed_by, ci.break_count, ci.total_break_minutes,
		       ci.last_break_start, ci.checked_in_at, ci.checked_out_at, ci.created_at, ci.updated_at,
		       s.seat_number
		FROM check_ins ci
		JOIN seats s ON s.id = ci.seat_id
		WHERE ci.id = $1
	`, id).Scan(
		&ci.ID, &ci.RegistrationID, &ci.UserID, &ci.SessionID, &ci.SeatID,
		&ci.Status, &ci.Method, &ci.ConfirmedBy, &ci.BreakCount, &ci.TotalBreakMinutes,
		&ci.LastBreakStart, &ci.CheckedInAt, &ci.CheckedOutAt, &ci.CreatedAt, &ci.UpdatedAt,
		&ci.SeatNumber,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find check_in: %w", err)
	}
	return ci, nil
}

// FindByRegistration returns a check-in by registration ID.
func (r *CheckInRepository) FindByRegistration(registrationID string) (*models.CheckIn, error) {
	ci := &models.CheckIn{}
	err := r.db.QueryRow(`
		SELECT ci.id, ci.registration_id, ci.user_id, ci.session_id, ci.seat_id,
		       ci.status, ci.method, ci.confirmed_by, ci.break_count, ci.total_break_minutes,
		       ci.last_break_start, ci.checked_in_at, ci.checked_out_at, ci.created_at, ci.updated_at
		FROM check_ins ci
		WHERE ci.registration_id = $1
	`, registrationID).Scan(
		&ci.ID, &ci.RegistrationID, &ci.UserID, &ci.SessionID, &ci.SeatID,
		&ci.Status, &ci.Method, &ci.ConfirmedBy, &ci.BreakCount, &ci.TotalBreakMinutes,
		&ci.LastBreakStart, &ci.CheckedInAt, &ci.CheckedOutAt, &ci.CreatedAt, &ci.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find check_in by registration: %w", err)
	}
	return ci, nil
}

// StartBreak transitions a check-in to on_break and updates the seat.
func (r *CheckInRepository) StartBreak(checkInID, seatID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE check_ins SET status = 'on_break', last_break_start = NOW(),
		    break_count = break_count + 1, updated_at = NOW()
		WHERE id = $1 AND status = 'active'
	`, checkInID)
	if err != nil {
		return fmt.Errorf("update check_in to on_break: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE seats SET status = 'on_break' WHERE id = $1
	`, seatID)
	if err != nil {
		return fmt.Errorf("update seat to on_break: %w", err)
	}

	return tx.Commit()
}

// ReturnFromBreak transitions back to active with break duration tracking.
func (r *CheckInRepository) ReturnFromBreak(checkInID, seatID string, breakMinutes int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE check_ins SET status = 'active',
		    total_break_minutes = total_break_minutes + $2,
		    updated_at = NOW()
		WHERE id = $1
	`, checkInID, breakMinutes)
	if err != nil {
		return fmt.Errorf("return from break: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE seats SET status = 'occupied' WHERE id = $1
	`, seatID)
	if err != nil {
		return fmt.Errorf("occupy seat: %w", err)
	}

	return tx.Commit()
}

// ReleaseSeatFromBreak releases a seat due to break overrun and restores session capacity.
func (r *CheckInRepository) ReleaseSeatFromBreak(checkInID, seatID, sessionID string, breakMinutes int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE check_ins SET status = 'released',
		    total_break_minutes = total_break_minutes + $2,
		    updated_at = NOW()
		WHERE id = $1
	`, checkInID, breakMinutes)
	if err != nil {
		return fmt.Errorf("release check_in: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE seats SET status = 'released', released_at = NOW(), assigned_user_id = NULL
		WHERE id = $1
	`, seatID)
	if err != nil {
		return fmt.Errorf("release seat: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE sessions SET available_seats = available_seats + 1, updated_at = NOW()
		WHERE id = $1
	`, sessionID)
	if err != nil {
		return fmt.Errorf("restore seat count: %w", err)
	}

	return tx.Commit()
}

// GetBreakMaxCount returns the configured max break count from config_entries.
func (r *CheckInRepository) GetBreakMaxCount() (int, error) {
	var val string
	err := r.db.QueryRow(`
		SELECT value FROM config_entries WHERE key = 'session.break_max_count'
	`).Scan(&val)
	if err != nil {
		return 2, nil // default
	}
	var n int
	fmt.Sscanf(val, "%d", &n)
	if n <= 0 {
		return 2, nil
	}
	return n, nil
}

// GetBreakMaxMinutes returns the configured max break duration from config_entries.
func (r *CheckInRepository) GetBreakMaxMinutes() (int, error) {
	var val string
	err := r.db.QueryRow(`
		SELECT value FROM config_entries WHERE key = 'session.break_max_minutes'
	`).Scan(&val)
	if err != nil {
		return 10, nil // default
	}
	var n int
	fmt.Sscanf(val, "%d", &n)
	if n <= 0 {
		return 10, nil
	}
	return n, nil
}
