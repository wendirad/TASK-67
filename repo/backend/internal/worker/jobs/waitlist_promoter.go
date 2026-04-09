package jobs

import (
	"context"
	"database/sql"
	"log"
)

const WaitlistPromoterLockID int64 = 100

// WaitlistPromoter promotes the next waitlisted user when a seat becomes available.
func WaitlistPromoter(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Find sessions with available seats and waiting waitlist entries
		rows, err := db.QueryContext(ctx, `
			SELECT DISTINCT w.session_id
			FROM waitlist w
			JOIN sessions s ON w.session_id = s.id
			WHERE w.status = 'waiting'
			AND s.available_seats > 0
			AND s.status IN ('open', 'closed', 'in_progress')
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		var sessionIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			sessionIDs = append(sessionIDs, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		for _, sessionID := range sessionIDs {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := promoteNext(ctx, db, sessionID); err != nil {
				log.Printf("Waitlist promoter: error promoting for session %s: %v", sessionID, err)
				// Continue with other sessions
			}
		}

		return nil
	}
}

func promoteNext(ctx context.Context, db *sql.DB, sessionID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Lock the session row
	var availableSeats int
	err = tx.QueryRowContext(ctx, `
		SELECT available_seats FROM sessions WHERE id = $1 FOR UPDATE
	`, sessionID).Scan(&availableSeats)
	if err != nil {
		return err
	}

	if availableSeats <= 0 {
		return nil // No seats left (race condition)
	}

	// Find the next waiting entry
	var waitlistID, userID string
	err = tx.QueryRowContext(ctx, `
		SELECT id, user_id FROM waitlist
		WHERE session_id = $1 AND status = 'waiting'
		ORDER BY position ASC LIMIT 1
		FOR UPDATE SKIP LOCKED
	`, sessionID).Scan(&waitlistID, &userID)
	if err == sql.ErrNoRows {
		return nil // No one waiting
	}
	if err != nil {
		return err
	}

	// Promote the waitlist entry
	_, err = tx.ExecContext(ctx, `
		UPDATE waitlist SET status = 'promoted', promoted_at = NOW()
		WHERE id = $1
	`, waitlistID)
	if err != nil {
		return err
	}

	// Update registration to registered
	_, err = tx.ExecContext(ctx, `
		UPDATE registrations SET status = 'registered', registered_at = NOW(), updated_at = NOW()
		WHERE user_id = $1 AND session_id = $2 AND status = 'waitlisted'
	`, userID, sessionID)
	if err != nil {
		return err
	}

	// Decrement available seats
	_, err = tx.ExecContext(ctx, `
		UPDATE sessions SET available_seats = available_seats - 1, updated_at = NOW()
		WHERE id = $1
	`, sessionID)
	if err != nil {
		return err
	}

	// Assign first available seat as reserved
	_, err = tx.ExecContext(ctx, `
		UPDATE seats SET status = 'reserved', assigned_user_id = $1, assigned_at = NOW()
		WHERE id = (
			SELECT id FROM seats WHERE session_id = $2 AND status = 'available'
			ORDER BY seat_number LIMIT 1 FOR UPDATE SKIP LOCKED
		)
	`, userID, sessionID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("Waitlist promoter: promoted user %s for session %s", userID, sessionID)
	return nil
}
