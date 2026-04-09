package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

const BreakOverrunDetectorLockID int64 = 103

// BreakOverrunDetector releases seats for check-ins on break beyond the configured limit.
// Runs every 15 seconds.
func BreakOverrunDetector(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Get break max minutes from config
		maxMinutes := 10
		var val string
		if err := db.QueryRowContext(ctx, `SELECT value FROM config_entries WHERE key = 'session.break_max_minutes'`).Scan(&val); err == nil {
			fmt.Sscanf(val, "%d", &maxMinutes)
		}

		rows, err := db.QueryContext(ctx, `
			SELECT ci.id, ci.seat_id, ci.session_id, ci.user_id
			FROM check_ins ci
			WHERE ci.status = 'on_break'
			AND ci.last_break_start < NOW() - ($1 || ' minutes')::interval
		`, maxMinutes)
		if err != nil {
			return err
		}
		defer rows.Close()

		type overrun struct {
			checkInID, seatID, sessionID, userID string
		}
		var overruns []overrun
		for rows.Next() {
			var o overrun
			if err := rows.Scan(&o.checkInID, &o.seatID, &o.sessionID, &o.userID); err != nil {
				return err
			}
			overruns = append(overruns, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		for _, o := range overruns {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := releaseBreakOverrun(ctx, db, o.checkInID, o.seatID, o.sessionID); err != nil {
				log.Printf("Break overrun detector: error releasing %s: %v", o.checkInID, err)
			}
		}

		return nil
	}
}

func releaseBreakOverrun(ctx context.Context, db *sql.DB, checkInID, seatID, sessionID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Release check-in (idempotent)
	result, err := tx.ExecContext(ctx, `
		UPDATE check_ins SET status = 'released', updated_at = NOW()
		WHERE id = $1 AND status = 'on_break'
	`, checkInID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil // Already handled
	}

	// Release seat
	_, err = tx.ExecContext(ctx, `
		UPDATE seats SET status = 'released', released_at = NOW(), assigned_user_id = NULL
		WHERE id = $1
	`, seatID)
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

	log.Printf("Break overrun detector: released check-in %s for session %s", checkInID, sessionID)
	return nil
}
