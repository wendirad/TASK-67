package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

const OccupancyAnomalyDetectorLockID int64 = 110

// OccupancyAnomalyDetector finds check-ins with prolonged unverified occupancy
// and generates staff-visible seat_exception tickets. It detects two anomalies:
//  1. Active check-ins where the session has ended (past end_time).
//  2. Active check-ins that have exceeded a configurable duration threshold
//     while the session is still in progress.
//
// Runs every 60 seconds.
func OccupancyAnomalyDetector(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		maxMinutes := 180
		var val string
		if err := db.QueryRowContext(ctx,
			`SELECT value FROM config_entries WHERE key = 'session.occupancy_max_minutes'`,
		).Scan(&val); err == nil {
			fmt.Sscanf(val, "%d", &maxMinutes)
		}

		// Find active check-ins that exceed the occupancy threshold OR whose
		// session has already ended, and for which no open seat_exception
		// ticket already exists.
		rows, err := db.QueryContext(ctx, `
			SELECT ci.id, ci.user_id, ci.session_id, ci.seat_id,
			       ci.checked_in_at, s.title, s.end_time,
			       u.username
			FROM check_ins ci
			JOIN sessions s ON ci.session_id = s.id
			JOIN users u ON ci.user_id = u.id
			WHERE ci.status = 'active'
			AND (
				s.end_time < NOW()
				OR ci.checked_in_at < NOW() - ($1 || ' minutes')::interval
			)
			AND NOT EXISTS (
				SELECT 1 FROM tickets t
				WHERE t.related_entity_type = 'check_in'
				AND t.related_entity_id = ci.id
				AND t.type = 'seat_exception'
				AND t.status NOT IN ('resolved', 'closed')
			)
		`, maxMinutes)
		if err != nil {
			return err
		}
		defer rows.Close()

		type anomaly struct {
			checkInID  string
			userID     string
			sessionID  string
			seatID     string
			checkedIn  time.Time
			sessionEnd time.Time
			title      string
			username   string
		}
		var anomalies []anomaly
		for rows.Next() {
			var a anomaly
			if err := rows.Scan(&a.checkInID, &a.userID, &a.sessionID, &a.seatID,
				&a.checkedIn, &a.title, &a.sessionEnd, &a.username); err != nil {
				return err
			}
			anomalies = append(anomalies, a)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		for _, a := range anomalies {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := createOccupancyTicket(ctx, db, a.checkInID, a.userID, a.sessionID,
				a.seatID, a.checkedIn, a.sessionEnd, a.title, a.username); err != nil {
				log.Printf("Occupancy anomaly detector: error creating ticket for check-in %s: %v", a.checkInID, err)
			}
		}

		return nil
	}
}

func createOccupancyTicket(ctx context.Context, db *sql.DB, checkInID, userID, sessionID, seatID string,
	checkedIn, sessionEnd time.Time, sessionTitle, username string) error {

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Double-check no duplicate ticket (guard against race)
	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM tickets
			WHERE related_entity_type = 'check_in'
			AND related_entity_id = $1
			AND type = 'seat_exception'
			AND status NOT IN ('resolved', 'closed')
		)
	`, checkInID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Resolve admin user for created_by (system-generated ticket)
	var adminID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1`,
	).Scan(&adminID)
	if err != nil {
		return fmt.Errorf("no admin user found: %w", err)
	}

	now := time.Now()
	duration := now.Sub(checkedIn).Truncate(time.Minute)
	ticketNumber := fmt.Sprintf("TKT-%s-%05d", now.Format("20060102"), now.UnixNano()%100000)

	pastEnd := now.After(sessionEnd)
	var subject, description string
	if pastEnd {
		subject = fmt.Sprintf("Prolonged occupancy: %s still checked in after session ended", username)
		description = fmt.Sprintf(
			"User %s has been checked in for %s in session \"%s\" but the session ended at %s. "+
				"The seat remains occupied without checkout. Staff should verify whether the user is still present and release the seat if appropriate.",
			username, duration, sessionTitle, sessionEnd.Format("2006-01-02 15:04 UTC"),
		)
	} else {
		subject = fmt.Sprintf("Prolonged occupancy: %s checked in for %s", username, duration)
		description = fmt.Sprintf(
			"User %s has been continuously checked in for %s in session \"%s\" (started at %s) without any break or checkout. "+
				"This exceeds the expected occupancy duration. Staff should verify the user's presence.",
			username, duration, sessionTitle, checkedIn.Format("2006-01-02 15:04 UTC"),
		)
	}

	// Calculate SLA deadlines
	slaResponseHours := 4
	slaResolutionDays := 3
	var slaVal string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM config_entries WHERE key = 'ticket.sla_response_hours'`).Scan(&slaVal); err == nil {
		fmt.Sscanf(slaVal, "%d", &slaResponseHours)
	}
	if err := tx.QueryRowContext(ctx, `SELECT value FROM config_entries WHERE key = 'ticket.sla_resolution_days'`).Scan(&slaVal); err == nil {
		fmt.Sscanf(slaVal, "%d", &slaResolutionDays)
	}
	slaResponseDeadline := calculateBusinessHourDeadline(now, slaResponseHours)
	slaResolutionDeadline := now.Add(time.Duration(slaResolutionDays) * 24 * time.Hour)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tickets (ticket_number, type, subject, description, status, priority,
		    created_by, related_entity_type, related_entity_id,
		    sla_response_deadline, sla_resolution_deadline)
		VALUES ($1, 'seat_exception', $2, $3, 'open', 'high', $4, 'check_in', $5, $6, $7)
	`, ticketNumber, subject, description, adminID, checkInID, slaResponseDeadline, slaResolutionDeadline)
	if err != nil {
		return fmt.Errorf("insert ticket: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("Occupancy anomaly detector: created ticket %s for check-in %s (user=%s session=%s)",
		ticketNumber, checkInID, username, sessionTitle)
	return nil
}

// calculateBusinessHourDeadline mirrors the SLA calculation from ticket repository.
func calculateBusinessHourDeadline(from time.Time, hours int) time.Time {
	remaining := time.Duration(hours) * time.Hour
	current := from

	for remaining > 0 {
		for current.Weekday() == time.Saturday || current.Weekday() == time.Sunday {
			current = time.Date(current.Year(), current.Month(), current.Day()+1, 9, 0, 0, 0, current.Location())
		}

		businessStart := time.Date(current.Year(), current.Month(), current.Day(), 9, 0, 0, 0, current.Location())
		businessEnd := time.Date(current.Year(), current.Month(), current.Day(), 18, 0, 0, 0, current.Location())

		if current.Before(businessStart) {
			current = businessStart
		}
		if !current.Before(businessEnd) {
			current = time.Date(current.Year(), current.Month(), current.Day()+1, 9, 0, 0, 0, current.Location())
			continue
		}

		available := businessEnd.Sub(current)
		if remaining <= available {
			return current.Add(remaining)
		}

		remaining -= available
		current = time.Date(current.Year(), current.Month(), current.Day()+1, 9, 0, 0, 0, current.Location())
	}

	return current
}
