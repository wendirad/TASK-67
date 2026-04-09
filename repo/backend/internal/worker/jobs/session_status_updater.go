package jobs

import (
	"context"
	"database/sql"
	"log"
)

const SessionStatusUpdaterLockID int64 = 104

// SessionStatusUpdater transitions session statuses based on time.
// Runs every 60 seconds. Handles:
//   - open → in_progress when start_time has passed
//   - in_progress → completed when end_time has passed (with cleanup)
func SessionStatusUpdater(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// 1. Start sessions: open → in_progress
		result, err := db.ExecContext(ctx, `
			UPDATE sessions SET status = 'in_progress', updated_at = NOW()
			WHERE status = 'open' AND start_time <= NOW()
		`)
		if err != nil {
			return err
		}
		if started, _ := result.RowsAffected(); started > 0 {
			log.Printf("Session updater: started %d session(s)", started)
		}

		// 2. Complete sessions: in_progress → completed
		rows, err := db.QueryContext(ctx, `
			SELECT id FROM sessions
			WHERE status = 'in_progress' AND end_time < NOW()
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
			if err := completeSession(ctx, db, sessionID); err != nil {
				log.Printf("Session updater: error completing session %s: %v", sessionID, err)
			}
		}

		return nil
	}
}

func completeSession(ctx context.Context, db *sql.DB, sessionID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Transition session to completed
	result, err := tx.ExecContext(ctx, `
		UPDATE sessions SET status = 'completed', updated_at = NOW()
		WHERE id = $1 AND status = 'in_progress'
	`, sessionID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil
	}

	// Complete all active check-ins
	_, err = tx.ExecContext(ctx, `
		UPDATE check_ins SET status = 'completed', checked_out_at = NOW(), updated_at = NOW()
		WHERE session_id = $1 AND status IN ('active', 'on_break')
	`, sessionID)
	if err != nil {
		return err
	}

	// Release all occupied/reserved/on_break seats
	_, err = tx.ExecContext(ctx, `
		UPDATE seats SET status = 'released', released_at = NOW()
		WHERE session_id = $1 AND status IN ('occupied', 'reserved', 'on_break')
	`, sessionID)
	if err != nil {
		return err
	}

	// Mark remaining registered registrations as completed
	_, err = tx.ExecContext(ctx, `
		UPDATE registrations SET status = 'completed', updated_at = NOW()
		WHERE session_id = $1 AND status = 'registered'
	`, sessionID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("Session updater: completed session %s", sessionID)
	return nil
}
