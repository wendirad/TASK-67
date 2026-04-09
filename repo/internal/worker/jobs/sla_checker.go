package jobs

import (
	"context"
	"database/sql"
	"log"
)

const SLACheckerLockID int64 = 105

// SLAChecker checks for breached SLA deadlines and marks them accordingly.
func SLAChecker(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Mark breached response SLAs
		result1, err := db.ExecContext(ctx, `
			UPDATE tickets SET sla_response_met = false, updated_at = NOW()
			WHERE status IN ('open', 'assigned')
			    AND sla_response_deadline < NOW()
			    AND responded_at IS NULL
			    AND (sla_response_met IS NULL OR sla_response_met = true)
		`)
		if err != nil {
			return err
		}
		responseBreached, _ := result1.RowsAffected()

		// Mark breached resolution SLAs
		result2, err := db.ExecContext(ctx, `
			UPDATE tickets SET sla_resolution_met = false, updated_at = NOW()
			WHERE status NOT IN ('resolved', 'closed')
			    AND sla_resolution_deadline < NOW()
			    AND resolved_at IS NULL
			    AND (sla_resolution_met IS NULL OR sla_resolution_met = true)
		`)
		if err != nil {
			return err
		}
		resolutionBreached, _ := result2.RowsAffected()

		if responseBreached > 0 || resolutionBreached > 0 {
			log.Printf("SLA checker: %d response breaches, %d resolution breaches",
				responseBreached, resolutionBreached)
		}

		return nil
	}
}
