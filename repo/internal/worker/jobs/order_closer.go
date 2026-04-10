package jobs

import (
	"context"
	"database/sql"
	"log"
)

const OrderCloserLockID int64 = 101

// OrderCloser closes expired orders that have passed their payment deadline.
// Each order is closed in its own transaction with atomic stock restoration.
// Uses rowsAffected to prevent double restoration on concurrent execution.
func OrderCloser(db *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Find expired orders (outside transaction scope)
		rows, err := db.QueryContext(ctx, `
			SELECT id FROM orders
			WHERE status = 'pending_payment' AND payment_deadline < NOW()
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		var orderIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			orderIDs = append(orderIDs, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		if len(orderIDs) == 0 {
			return nil
		}

		closed := 0
		for _, orderID := range orderIDs {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			ok, err := closeExpiredOrder(ctx, db, orderID)
			if err != nil {
				log.Printf("Order closer: error closing order %s: %v", orderID, err)
				continue
			}
			if ok {
				closed++
			}
		}

		if closed > 0 {
			log.Printf("Order closer: closed %d expired order(s)", closed)
		}
		return nil
	}
}

func closeExpiredOrder(ctx context.Context, db *sql.DB, orderID string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Close the order — capture affected row count
	result, err := tx.ExecContext(ctx, `
		UPDATE orders SET status = 'closed', closed_at = NOW(),
		    close_reason = 'Payment timeout - deadline exceeded',
		    updated_at = NOW()
		WHERE id = $1 AND status = 'pending_payment'
	`, orderID)
	if err != nil {
		return false, err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		// Already closed by another worker or retry — skip stock restore
		return false, nil
	}

	// Restore stock only if the state transition actually happened
	_, err = tx.ExecContext(ctx, `
		UPDATE products p
		SET stock_quantity = p.stock_quantity + oi.quantity, updated_at = NOW()
		FROM order_items oi
		WHERE oi.order_id = $1 AND oi.product_id = p.id
	`, orderID)
	if err != nil {
		return false, err
	}

	// Cancel associated pending payment
	_, err = tx.ExecContext(ctx, `
		UPDATE payments SET status = 'failed', updated_at = NOW()
		WHERE order_id = $1 AND status = 'pending'
	`, orderID)
	if err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	log.Printf("Order closer: closed expired order %s", orderID)
	return true, nil
}
