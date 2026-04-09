package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

const NoShowDetectorLockID int64 = 102

// NoShowDetector marks no-show registrations and releases their seats.
// Runs every 30 seconds. Finds registrations in 'registered' state with no check-in
// for sessions that started more than 10 minutes ago.
func NoShowDetector(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Get noshow minutes from config
		noshowMinutes := 10
		var val string
		if err := db.QueryRowContext(ctx, `SELECT value FROM config_entries WHERE key = 'session.noshow_minutes'`).Scan(&val); err == nil {
			fmt.Sscanf(val, "%d", &noshowMinutes)
		}

		rows, err := db.QueryContext(ctx, `
			SELECT r.id, r.session_id, r.user_id
			FROM registrations r
			JOIN sessions s ON r.session_id = s.id
			WHERE r.status = 'registered'
			AND s.start_time < NOW() - ($1 || ' minutes')::interval
			AND s.status = 'in_progress'
			AND NOT EXISTS (
				SELECT 1 FROM check_ins ci WHERE ci.registration_id = r.id
			)
		`, noshowMinutes)
		if err != nil {
			return err
		}
		defer rows.Close()

		type noshow struct {
			regID, sessionID, userID string
		}
		var noshows []noshow
		for rows.Next() {
			var ns noshow
			if err := rows.Scan(&ns.regID, &ns.sessionID, &ns.userID); err != nil {
				return err
			}
			noshows = append(noshows, ns)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		for _, ns := range noshows {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := markNoShow(ctx, db, ns.regID, ns.sessionID, ns.userID); err != nil {
				log.Printf("No-show detector: error processing reg %s: %v", ns.regID, err)
			}
		}

		return nil
	}
}

func markNoShow(ctx context.Context, db *sql.DB, regID, sessionID, userID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Mark registration as no_show (idempotent: only if still registered)
	result, err := tx.ExecContext(ctx, `
		UPDATE registrations SET status = 'no_show', updated_at = NOW()
		WHERE id = $1 AND status = 'registered'
	`, regID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil // Already handled
	}

	// Release the reserved seat
	_, err = tx.ExecContext(ctx, `
		UPDATE seats SET status = 'released', released_at = NOW(), assigned_user_id = NULL
		WHERE session_id = $1 AND assigned_user_id = $2 AND status = 'reserved'
	`, sessionID, userID)
	if err != nil {
		return err
	}

	// Restore available seat
	_, err = tx.ExecContext(ctx, `
		UPDATE sessions SET available_seats = available_seats + 1, updated_at = NOW()
		WHERE id = $1
	`, sessionID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("No-show detector: marked reg %s user %s for session %s", regID, userID, sessionID)
	return nil
}
