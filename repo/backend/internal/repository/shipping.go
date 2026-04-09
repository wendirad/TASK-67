package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type ShippingRepository struct {
	db *sql.DB
}

func NewShippingRepository(db *sql.DB) *ShippingRepository {
	return &ShippingRepository{db: db}
}

// Create inserts a new shipping record.
func (r *ShippingRepository) Create(tx *sql.Tx, orderID string) error {
	_, err := tx.Exec(`
		INSERT INTO shipping_records (order_id, status)
		VALUES ($1, 'pending')
	`, orderID)
	if err != nil {
		return fmt.Errorf("create shipping record: %w", err)
	}
	return nil
}

// FindByID returns a shipping record by ID.
func (r *ShippingRepository) FindByID(id string) (*models.ShippingRecord, error) {
	sr := &models.ShippingRecord{}
	err := r.db.QueryRow(`
		SELECT id, order_id, tracking_number, carrier, status,
		       shipped_at, delivered_at, proof_type, proof_data, exception_notes,
		       handled_by, created_at, updated_at
		FROM shipping_records WHERE id = $1
	`, id).Scan(
		&sr.ID, &sr.OrderID, &sr.TrackingNumber, &sr.Carrier, &sr.Status,
		&sr.ShippedAt, &sr.DeliveredAt, &sr.ProofType, &sr.ProofData, &sr.ExceptionNotes,
		&sr.HandledBy, &sr.CreatedAt, &sr.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find shipping record: %w", err)
	}
	return sr, nil
}

// FindByOrderID returns a shipping record by order ID.
func (r *ShippingRepository) FindByOrderID(orderID string) (*models.ShippingRecord, error) {
	sr := &models.ShippingRecord{}
	err := r.db.QueryRow(`
		SELECT id, order_id, tracking_number, carrier, status,
		       shipped_at, delivered_at, proof_type, proof_data, exception_notes,
		       handled_by, created_at, updated_at
		FROM shipping_records WHERE order_id = $1
	`, orderID).Scan(
		&sr.ID, &sr.OrderID, &sr.TrackingNumber, &sr.Carrier, &sr.Status,
		&sr.ShippedAt, &sr.DeliveredAt, &sr.ProofType, &sr.ProofData, &sr.ExceptionNotes,
		&sr.HandledBy, &sr.CreatedAt, &sr.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find shipping by order: %w", err)
	}
	return sr, nil
}

// ListAll returns paginated shipping records with order info for staff/admin.
func (r *ShippingRepository) ListAll(page, pageSize int, status, orderNumber string) ([]models.ShippingRecord, int, error) {
	baseQuery := `FROM shipping_records sr
		JOIN orders o ON o.id = sr.order_id
		JOIN users u ON u.id = o.user_id
		WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		baseQuery += fmt.Sprintf(` AND sr.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if orderNumber != "" {
		baseQuery += fmt.Sprintf(` AND o.order_number ILIKE $%d`, argIdx)
		args = append(args, "%"+orderNumber+"%")
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count shipping records: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT sr.id, sr.order_id, sr.tracking_number, sr.carrier, sr.status,
		       sr.shipped_at, sr.delivered_at, sr.proof_type, sr.proof_data, sr.exception_notes,
		       sr.handled_by, sr.created_at, sr.updated_at,
		       o.order_number, u.username, u.display_name
		%s ORDER BY sr.created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list shipping records: %w", err)
	}
	defer rows.Close()

	var records []models.ShippingRecord
	for rows.Next() {
		var sr models.ShippingRecord
		if err := rows.Scan(
			&sr.ID, &sr.OrderID, &sr.TrackingNumber, &sr.Carrier, &sr.Status,
			&sr.ShippedAt, &sr.DeliveredAt, &sr.ProofType, &sr.ProofData, &sr.ExceptionNotes,
			&sr.HandledBy, &sr.CreatedAt, &sr.UpdatedAt,
			&sr.OrderNumber, &sr.Username, &sr.DisplayName,
		); err != nil {
			return nil, 0, fmt.Errorf("scan shipping record: %w", err)
		}
		records = append(records, sr)
	}
	return records, total, rows.Err()
}

// Ship marks a shipping record as shipped and updates the order status.
func (r *ShippingRepository) Ship(id, staffID string, trackingNumber, carrier *string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Update shipping record
	result, err := tx.Exec(`
		UPDATE shipping_records SET status = 'shipped', shipped_at = NOW(),
		    tracking_number = $2, carrier = $3, handled_by = $4, updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
	`, id, trackingNumber, carrier, staffID)
	if err != nil {
		return fmt.Errorf("update shipping: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("shipping record not in pending state")
	}

	// Get order ID and update order status
	var orderID string
	if err := tx.QueryRow(`SELECT order_id FROM shipping_records WHERE id = $1`, id).Scan(&orderID); err != nil {
		return fmt.Errorf("get order id: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE orders SET status = 'shipped', updated_at = NOW()
		WHERE id = $1 AND status = 'paid'
	`, orderID)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	return tx.Commit()
}

// Deliver marks a shipping record as delivered with proof and updates the order.
func (r *ShippingRepository) Deliver(id, staffID, proofType, proofData string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE shipping_records SET status = 'delivered', delivered_at = NOW(),
		    proof_type = $2, proof_data = $3, handled_by = $4, updated_at = NOW()
		WHERE id = $1 AND status IN ('shipped', 'in_transit')
	`, id, proofType, proofData, staffID)
	if err != nil {
		return fmt.Errorf("update shipping delivery: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("shipping record not in shipped/in_transit state")
	}

	var orderID string
	if err := tx.QueryRow(`SELECT order_id FROM shipping_records WHERE id = $1`, id).Scan(&orderID); err != nil {
		return fmt.Errorf("get order id: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE orders SET status = 'delivered', updated_at = NOW()
		WHERE id = $1 AND status = 'shipped'
	`, orderID)
	if err != nil {
		return fmt.Errorf("update order to delivered: %w", err)
	}

	return tx.Commit()
}

// MarkException marks a shipping record with an exception.
func (r *ShippingRepository) MarkException(id, staffID, notes string) error {
	result, err := r.db.Exec(`
		UPDATE shipping_records SET status = 'exception', exception_notes = $2,
		    handled_by = $3, updated_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'shipped', 'in_transit')
	`, id, notes, staffID)
	if err != nil {
		return fmt.Errorf("mark shipping exception: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("shipping record cannot be marked as exception in current state")
	}
	return nil
}

// CompleteOrder transitions a delivered order to completed.
func (r *ShippingRepository) CompleteOrder(orderID string) error {
	result, err := r.db.Exec(`
		UPDATE orders SET status = 'completed', updated_at = NOW()
		WHERE id = $1 AND status = 'delivered'
	`, orderID)
	if err != nil {
		return fmt.Errorf("complete order: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("order not in delivered state")
	}
	return nil
}

// DB returns the underlying database connection for transaction use.
func (r *ShippingRepository) DB() *sql.DB {
	return r.db
}
