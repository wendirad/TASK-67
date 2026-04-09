package repository

import (
	"database/sql"
	"fmt"
	"time"

	"campusrec/internal/models"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) List(page, pageSize int, status, facility, search, fromDate, toDate string) ([]models.Session, int, error) {
	baseQuery := `FROM sessions s JOIN facilities f ON s.facility_id = f.id WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		baseQuery += fmt.Sprintf(` AND s.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if facility != "" {
		baseQuery += fmt.Sprintf(` AND (f.name ILIKE $%d OR f.id::text = $%d)`, argIdx, argIdx)
		args = append(args, "%"+facility+"%")
		argIdx++
	}
	if search != "" {
		baseQuery += fmt.Sprintf(` AND (s.title ILIKE $%d OR s.coach_name ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if fromDate != "" {
		baseQuery += fmt.Sprintf(` AND s.start_time >= $%d`, argIdx)
		args = append(args, fromDate)
		argIdx++
	}
	if toDate != "" {
		baseQuery += fmt.Sprintf(` AND s.start_time <= $%d`, argIdx)
		args = append(args, toDate)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT s.id, s.title, s.description, s.coach_name, s.facility_id, f.name,
		       s.start_time, s.end_time, s.total_seats, s.available_seats,
		       s.registration_close_before_minutes, s.status, s.created_by,
		       s.created_at, s.updated_at
		%s ORDER BY s.start_time ASC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	var sessions []models.Session
	for rows.Next() {
		var s models.Session
		if err := rows.Scan(
			&s.ID, &s.Title, &s.Description, &s.CoachName, &s.FacilityID, &s.FacilityName,
			&s.StartTime, &s.EndTime, &s.TotalSeats, &s.AvailableSeats,
			&s.RegistrationCloseBeforeMin, &s.Status, &s.CreatedBy,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan session: %w", err)
		}
		s.RegistrationOpen = s.Status == "open" &&
			now.Before(s.StartTime.Add(-time.Duration(s.RegistrationCloseBeforeMin)*time.Minute))
		sessions = append(sessions, s)
	}
	return sessions, total, rows.Err()
}

func (r *SessionRepository) FindByID(id string) (*models.Session, error) {
	s := &models.Session{}
	err := r.db.QueryRow(`
		SELECT s.id, s.title, s.description, s.coach_name, s.facility_id, f.name,
		       s.start_time, s.end_time, s.total_seats, s.available_seats,
		       s.registration_close_before_minutes, s.status, s.created_by,
		       s.created_at, s.updated_at
		FROM sessions s JOIN facilities f ON s.facility_id = f.id
		WHERE s.id = $1
	`, id).Scan(
		&s.ID, &s.Title, &s.Description, &s.CoachName, &s.FacilityID, &s.FacilityName,
		&s.StartTime, &s.EndTime, &s.TotalSeats, &s.AvailableSeats,
		&s.RegistrationCloseBeforeMin, &s.Status, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find session: %w", err)
	}
	now := time.Now()
	s.RegistrationOpen = s.Status == "open" &&
		now.Before(s.StartTime.Add(-time.Duration(s.RegistrationCloseBeforeMin)*time.Minute))
	return s, nil
}

func (r *SessionRepository) Create(s *models.Session) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRow(`
		INSERT INTO sessions (title, description, coach_name, facility_id, start_time, end_time,
		                      total_seats, available_seats, registration_close_before_minutes, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8, 'open', $9)
		RETURNING id, available_seats, status, created_at, updated_at
	`, s.Title, s.Description, s.CoachName, s.FacilityID, s.StartTime, s.EndTime,
		s.TotalSeats, s.RegistrationCloseBeforeMin, s.CreatedBy,
	).Scan(&s.ID, &s.AvailableSeats, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	for i := 1; i <= s.TotalSeats; i++ {
		_, err := tx.Exec(`
			INSERT INTO seats (session_id, seat_number, status) VALUES ($1, $2, 'available')
		`, s.ID, i)
		if err != nil {
			return fmt.Errorf("insert seat %d: %w", i, err)
		}
	}

	return tx.Commit()
}

func (r *SessionRepository) Update(s *models.Session) error {
	return r.db.QueryRow(`
		UPDATE sessions
		SET title = $1, description = $2, coach_name = $3, facility_id = $4,
		    start_time = $5, end_time = $6, total_seats = $7, available_seats = $8,
		    registration_close_before_minutes = $9, updated_at = NOW()
		WHERE id = $10
		RETURNING updated_at
	`, s.Title, s.Description, s.CoachName, s.FacilityID,
		s.StartTime, s.EndTime, s.TotalSeats, s.AvailableSeats,
		s.RegistrationCloseBeforeMin, s.ID,
	).Scan(&s.UpdatedAt)
}

func (r *SessionRepository) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(`
		UPDATE sessions SET status = $1, updated_at = NOW() WHERE id = $2
	`, status, id)
	return err
}

// CountCapacityConsumingSeats returns the count of seats in reserved, occupied, or on_break status.
func (r *SessionRepository) CountCapacityConsumingSeats(sessionID string) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM seats
		WHERE session_id = $1 AND status IN ('reserved', 'occupied', 'on_break')
	`, sessionID).Scan(&count)
	return count, err
}

// CancelSession atomically cancels a session, releasing all seats, canceling registrations and waitlist.
func (r *SessionRepository) CancelSession(sessionID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE sessions SET status = 'canceled', updated_at = NOW() WHERE id = $1`, sessionID)
	if err != nil {
		return fmt.Errorf("cancel session: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE seats SET status = 'released', released_at = NOW()
		WHERE session_id = $1 AND status IN ('available', 'reserved', 'occupied', 'on_break')
	`, sessionID)
	if err != nil {
		return fmt.Errorf("release seats: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE registrations SET status = 'canceled', updated_at = NOW()
		WHERE session_id = $1 AND status IN ('pending', 'approved', 'registered', 'waitlisted')
	`, sessionID)
	if err != nil {
		return fmt.Errorf("cancel registrations: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE waitlist SET status = 'canceled'
		WHERE session_id = $1 AND status = 'waiting'
	`, sessionID)
	if err != nil {
		return fmt.Errorf("cancel waitlist: %w", err)
	}

	return tx.Commit()
}

// AddSeats adds new seat records when total_seats is increased.
func (r *SessionRepository) AddSeats(tx *sql.Tx, sessionID string, fromNumber, toNumber int) error {
	for i := fromNumber; i <= toNumber; i++ {
		_, err := tx.Exec(`
			INSERT INTO seats (session_id, seat_number, status) VALUES ($1, $2, 'available')
		`, sessionID, i)
		if err != nil {
			return fmt.Errorf("insert seat %d: %w", i, err)
		}
	}
	return nil
}

// UpdateWithSeats updates session and manages seat records in a transaction.
func (r *SessionRepository) UpdateWithSeats(s *models.Session, oldTotalSeats int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRow(`
		UPDATE sessions
		SET title = $1, description = $2, coach_name = $3, facility_id = $4,
		    start_time = $5, end_time = $6, total_seats = $7, available_seats = $8,
		    registration_close_before_minutes = $9, updated_at = NOW()
		WHERE id = $10
		RETURNING updated_at
	`, s.Title, s.Description, s.CoachName, s.FacilityID,
		s.StartTime, s.EndTime, s.TotalSeats, s.AvailableSeats,
		s.RegistrationCloseBeforeMin, s.ID,
	).Scan(&s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	if s.TotalSeats > oldTotalSeats {
		if err := r.AddSeats(tx, s.ID, oldTotalSeats+1, s.TotalSeats); err != nil {
			return err
		}
	}

	return tx.Commit()
}
