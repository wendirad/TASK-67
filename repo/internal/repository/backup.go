package repository

import (
	"database/sql"
	"fmt"
	"time"

	"campusrec/internal/models"
)

type BackupRepository struct {
	db *sql.DB
}

func NewBackupRepository(db *sql.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

// CreateBackup inserts a new backup record.
func (r *BackupRepository) CreateBackup(filename, backupType string, encrypted bool) (*models.Backup, error) {
	b := &models.Backup{}
	err := r.db.QueryRow(`
		INSERT INTO backups (filename, size_bytes, encrypted, type, status)
		VALUES ($1, 0, $3, $2, 'in_progress')
		RETURNING id, filename, size_bytes, encrypted, type, status, wal_start_lsn, started_at, completed_at, created_at
	`, filename, backupType, encrypted).Scan(
		&b.ID, &b.Filename, &b.SizeBytes, &b.Encrypted, &b.Type,
		&b.Status, &b.WALStartLSN, &b.StartedAt, &b.CompletedAt, &b.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
	}
	return b, nil
}

// FindPendingBackups returns all backups with status 'in_progress'.
func (r *BackupRepository) FindPendingBackups() ([]models.Backup, error) {
	rows, err := r.db.Query(`
		SELECT id, filename, size_bytes, encrypted, type, status, wal_start_lsn, started_at, completed_at, created_at
		FROM backups
		WHERE status = 'in_progress'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("find pending backups: %w", err)
	}
	defer rows.Close()

	var backups []models.Backup
	for rows.Next() {
		var b models.Backup
		if err := rows.Scan(&b.ID, &b.Filename, &b.SizeBytes, &b.Encrypted, &b.Type,
			&b.Status, &b.WALStartLSN, &b.StartedAt, &b.CompletedAt, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending backup: %w", err)
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

// GetCurrentWALLSN returns the current PostgreSQL WAL log sequence number.
func (r *BackupRepository) GetCurrentWALLSN() (string, error) {
	var lsn string
	err := r.db.QueryRow(`SELECT pg_current_wal_lsn()::text`).Scan(&lsn)
	if err != nil {
		return "", fmt.Errorf("get current WAL LSN: %w", err)
	}
	return lsn, nil
}

// CompleteBackup marks a backup as completed with file size and WAL LSN.
func (r *BackupRepository) CompleteBackup(id string, sizeBytes int64, walStartLSN string) error {
	_, err := r.db.Exec(`
		UPDATE backups SET status = 'completed', size_bytes = $2, wal_start_lsn = $3, completed_at = NOW()
		WHERE id = $1
	`, id, sizeBytes, walStartLSN)
	if err != nil {
		return fmt.Errorf("complete backup: %w", err)
	}
	return nil
}

// FailBackup marks a backup as failed.
func (r *BackupRepository) FailBackup(id string) error {
	_, err := r.db.Exec(`UPDATE backups SET status = 'failed', completed_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("fail backup: %w", err)
	}
	return nil
}

// ListBackups returns all backups ordered by creation date descending.
func (r *BackupRepository) ListBackups() ([]models.Backup, error) {
	rows, err := r.db.Query(`
		SELECT id, filename, size_bytes, encrypted, type, status, wal_start_lsn, started_at, completed_at, created_at
		FROM backups
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	defer rows.Close()

	var backups []models.Backup
	for rows.Next() {
		var b models.Backup
		if err := rows.Scan(&b.ID, &b.Filename, &b.SizeBytes, &b.Encrypted, &b.Type,
			&b.Status, &b.WALStartLSN, &b.StartedAt, &b.CompletedAt, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan backup: %w", err)
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

// FindByID returns a backup by ID.
func (r *BackupRepository) FindByID(id string) (*models.Backup, error) {
	b := &models.Backup{}
	err := r.db.QueryRow(`
		SELECT id, filename, size_bytes, encrypted, type, status, wal_start_lsn, started_at, completed_at, created_at
		FROM backups WHERE id = $1
	`, id).Scan(&b.ID, &b.Filename, &b.SizeBytes, &b.Encrypted, &b.Type,
		&b.Status, &b.WALStartLSN, &b.StartedAt, &b.CompletedAt, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find backup: %w", err)
	}
	return b, nil
}

// GetRestoreTargets returns the available restore window.
func (r *BackupRepository) GetRestoreTargets() (*models.RestoreTargets, error) {
	targets := &models.RestoreTargets{
		BaseBackups: []models.BackupSummary{},
	}

	// Get earliest completed backup
	err := r.db.QueryRow(`
		SELECT started_at FROM backups WHERE status = 'completed'
		ORDER BY started_at ASC LIMIT 1
	`).Scan(&targets.EarliestSnapshot)
	if err == sql.ErrNoRows {
		return targets, nil
	}
	if err != nil {
		return nil, fmt.Errorf("earliest snapshot: %w", err)
	}

	targets.EarliestPITR = targets.EarliestSnapshot
	now := time.Now()
	targets.LatestPITR = &now

	// Get all completed backups as summaries
	rows, err := r.db.Query(`
		SELECT id, started_at, size_bytes FROM backups
		WHERE status = 'completed'
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list backup summaries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s models.BackupSummary
		if err := rows.Scan(&s.ID, &s.Timestamp, &s.SizeBytes); err != nil {
			return nil, fmt.Errorf("scan backup summary: %w", err)
		}
		targets.BaseBackups = append(targets.BaseBackups, s)
	}
	return targets, rows.Err()
}

// FindBaseBackupForPITR finds the most recent completed backup before the target time.
func (r *BackupRepository) FindBaseBackupForPITR(targetTime time.Time) (*models.Backup, error) {
	b := &models.Backup{}
	err := r.db.QueryRow(`
		SELECT id, filename, size_bytes, encrypted, type, status, wal_start_lsn, started_at, completed_at, created_at
		FROM backups
		WHERE status = 'completed' AND started_at <= $1
		ORDER BY started_at DESC LIMIT 1
	`, targetTime).Scan(&b.ID, &b.Filename, &b.SizeBytes, &b.Encrypted, &b.Type,
		&b.Status, &b.WALStartLSN, &b.StartedAt, &b.CompletedAt, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find base backup for pitr: %w", err)
	}
	return b, nil
}

// DeleteOldBackups removes backups older than the retention period.
func (r *BackupRepository) DeleteOldBackups(olderThan time.Time) (int64, error) {
	result, err := r.db.Exec(`
		DELETE FROM backups WHERE created_at < $1 AND status = 'completed'
	`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("delete old backups: %w", err)
	}
	return result.RowsAffected()
}

// ArchiveOrders archives orders older than the cutoff date in batched transactions.
// Returns number of orders archived.
func (r *BackupRepository) ArchiveOrders(cutoffMonths int, batchLimit int) (int, error) {
	cutoff := time.Now().AddDate(0, -cutoffMonths, 0)
	archiveTime := time.Now()

	// Find candidate orders
	rows, err := r.db.Query(`
		SELECT id FROM orders
		WHERE status IN ('completed', 'closed', 'refunded')
		  AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, cutoff, batchLimit)
	if err != nil {
		return 0, fmt.Errorf("find archive candidates: %w", err)
	}
	defer rows.Close()

	var orderIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan order id: %w", err)
		}
		orderIDs = append(orderIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	archived := 0
	for _, orderID := range orderIDs {
		tx, err := r.db.Begin()
		if err != nil {
			return archived, fmt.Errorf("begin tx: %w", err)
		}

		if err := r.archiveOrderTx(tx, orderID, archiveTime); err != nil {
			tx.Rollback()
			return archived, fmt.Errorf("archive order %s: %w", orderID, err)
		}

		if err := tx.Commit(); err != nil {
			return archived, fmt.Errorf("commit archive order %s: %w", orderID, err)
		}
		archived++
	}

	return archived, nil
}

func (r *BackupRepository) archiveOrderTx(tx *sql.Tx, orderID string, archiveTime time.Time) error {
	// Copy order to archive with PII masking
	_, err := tx.Exec(`
		INSERT INTO archive.orders (
			id, order_number, user_id, status, total_cents,
			shipping_address_id,
			ship_to_recipient, ship_to_phone,
			ship_to_line1, ship_to_line2, ship_to_city,
			ship_to_province, ship_to_postal_code,
			payment_deadline, paid_at, closed_at, close_reason,
			notes, created_at, updated_at, archived_at
		)
		SELECT
			id, order_number, user_id, status, total_cents,
			shipping_address_id,
			'ARCHIVED', NULL,
			ship_to_line1, ship_to_line2, ship_to_city,
			ship_to_province, ship_to_postal_code,
			payment_deadline, paid_at, closed_at, close_reason,
			NULL, created_at, updated_at, $2
		FROM orders WHERE id = $1
	`, orderID, archiveTime)
	if err != nil {
		return fmt.Errorf("copy order: %w", err)
	}

	// Copy order items
	_, err = tx.Exec(`
		INSERT INTO archive.order_items (
			id, order_id, product_id, product_name,
			quantity, unit_price_cents, total_cents, archived_at
		)
		SELECT
			id, order_id, product_id, product_name,
			quantity, unit_price_cents, total_cents, $2
		FROM order_items WHERE order_id = $1
	`, orderID, archiveTime)
	if err != nil {
		return fmt.Errorf("copy order items: %w", err)
	}

	// Copy payments
	_, err = tx.Exec(`
		INSERT INTO archive.payments (
			id, order_id, payment_method, amount_cents, status,
			transaction_id, wechat_prepay_data, callback_signature,
			callback_received_at, refund_id, refunded_at,
			created_at, updated_at, archived_at
		)
		SELECT
			id, order_id, payment_method, amount_cents, status,
			transaction_id, wechat_prepay_data, callback_signature,
			callback_received_at, refund_id, refunded_at,
			created_at, updated_at, $2
		FROM payments WHERE order_id = $1
	`, orderID, archiveTime)
	if err != nil {
		return fmt.Errorf("copy payments: %w", err)
	}

	// Copy related audit logs
	_, err = tx.Exec(`
		INSERT INTO archive.audit_logs (
			id, entity_type, entity_id, action,
			old_value, new_value, performed_by, ip_address,
			created_at, archived_at
		)
		SELECT
			id, entity_type, entity_id, action,
			old_value, new_value, performed_by, ip_address,
			created_at, $2
		FROM audit_logs
		WHERE entity_type = 'order' AND entity_id = $1
	`, orderID, archiveTime)
	if err != nil {
		return fmt.Errorf("copy audit logs: %w", err)
	}

	// Delete from public in FK-safe order (children first)
	_, err = tx.Exec(`DELETE FROM audit_logs WHERE entity_type = 'order' AND entity_id = $1`, orderID)
	if err != nil {
		return fmt.Errorf("delete audit logs: %w", err)
	}
	_, err = tx.Exec(`DELETE FROM payments WHERE order_id = $1`, orderID)
	if err != nil {
		return fmt.Errorf("delete payments: %w", err)
	}
	_, err = tx.Exec(`DELETE FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return fmt.Errorf("delete order items: %w", err)
	}
	_, err = tx.Exec(`DELETE FROM orders WHERE id = $1`, orderID)
	if err != nil {
		return fmt.Errorf("delete order: %w", err)
	}

	return nil
}

// ArchiveTickets archives closed tickets older than the cutoff date.
// Returns number of tickets archived.
func (r *BackupRepository) ArchiveTickets(cutoffMonths int, batchLimit int) (int, error) {
	cutoff := time.Now().AddDate(0, -cutoffMonths, 0)
	archiveTime := time.Now()

	rows, err := r.db.Query(`
		SELECT id FROM tickets
		WHERE status = 'closed'
		  AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, cutoff, batchLimit)
	if err != nil {
		return 0, fmt.Errorf("find ticket archive candidates: %w", err)
	}
	defer rows.Close()

	var ticketIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan ticket id: %w", err)
		}
		ticketIDs = append(ticketIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	archived := 0
	for _, ticketID := range ticketIDs {
		tx, err := r.db.Begin()
		if err != nil {
			return archived, fmt.Errorf("begin tx: %w", err)
		}

		if err := r.archiveTicketTx(tx, ticketID, archiveTime); err != nil {
			tx.Rollback()
			return archived, fmt.Errorf("archive ticket %s: %w", ticketID, err)
		}

		if err := tx.Commit(); err != nil {
			return archived, fmt.Errorf("commit archive ticket %s: %w", ticketID, err)
		}
		archived++
	}

	return archived, nil
}

func (r *BackupRepository) archiveTicketTx(tx *sql.Tx, ticketID string, archiveTime time.Time) error {
	// Copy ticket to archive with PII masking on description
	_, err := tx.Exec(`
		INSERT INTO archive.tickets (
			id, ticket_number, subject, description, type, priority,
			status, created_by, assigned_to,
			sla_response_deadline, sla_resolution_deadline,
			sla_response_met, sla_resolution_met,
			responded_at, resolved_at, closed_at,
			created_at, updated_at, archived_at
		)
		SELECT
			id, ticket_number, subject, 'ARCHIVED', type, priority,
			status, created_by, assigned_to,
			sla_response_deadline, sla_resolution_deadline,
			sla_response_met, sla_resolution_met,
			responded_at, resolved_at, closed_at,
			created_at, updated_at, $2
		FROM tickets WHERE id = $1
	`, ticketID, archiveTime)
	if err != nil {
		return fmt.Errorf("copy ticket: %w", err)
	}

	// Copy ticket comments with PII masking
	_, err = tx.Exec(`
		INSERT INTO archive.ticket_comments (
			id, ticket_id, user_id, content, created_at, archived_at
		)
		SELECT
			id, ticket_id, user_id, 'ARCHIVED', created_at, $2
		FROM ticket_comments WHERE ticket_id = $1
	`, ticketID, archiveTime)
	if err != nil {
		return fmt.Errorf("copy ticket comments: %w", err)
	}

	// Copy related audit logs
	_, err = tx.Exec(`
		INSERT INTO archive.audit_logs (
			id, entity_type, entity_id, action,
			old_value, new_value, performed_by, ip_address,
			created_at, archived_at
		)
		SELECT
			id, entity_type, entity_id, action,
			old_value, new_value, performed_by, ip_address,
			created_at, $2
		FROM audit_logs
		WHERE entity_type = 'ticket' AND entity_id = $1
	`, ticketID, archiveTime)
	if err != nil {
		return fmt.Errorf("copy ticket audit logs: %w", err)
	}

	// Delete from public in FK-safe order
	_, err = tx.Exec(`DELETE FROM audit_logs WHERE entity_type = 'ticket' AND entity_id = $1`, ticketID)
	if err != nil {
		return fmt.Errorf("delete ticket audit logs: %w", err)
	}
	_, err = tx.Exec(`DELETE FROM ticket_comments WHERE ticket_id = $1`, ticketID)
	if err != nil {
		return fmt.Errorf("delete ticket comments: %w", err)
	}
	_, err = tx.Exec(`DELETE FROM tickets WHERE id = $1`, ticketID)
	if err != nil {
		return fmt.Errorf("delete ticket: %w", err)
	}

	return nil
}

// GetArchiveStatus returns counts from archive tables and the most recent archived_at timestamp.
func (r *BackupRepository) GetArchiveStatus() (*models.ArchiveStatus, error) {
	status := &models.ArchiveStatus{}

	err := r.db.QueryRow(`SELECT COUNT(*) FROM archive.orders`).Scan(&status.OrdersArchived)
	if err != nil {
		return nil, fmt.Errorf("count archived orders: %w", err)
	}

	err = r.db.QueryRow(`SELECT COUNT(*) FROM archive.tickets`).Scan(&status.TicketsArchived)
	if err != nil {
		return nil, fmt.Errorf("count archived tickets: %w", err)
	}

	err = r.db.QueryRow(`SELECT COUNT(*) FROM archive.audit_logs`).Scan(&status.AuditLogsArchived)
	if err != nil {
		return nil, fmt.Errorf("count archived audit logs: %w", err)
	}

	status.TotalArchived = status.OrdersArchived + status.TicketsArchived + status.AuditLogsArchived

	// Get most recent archive timestamp
	err = r.db.QueryRow(`
		SELECT MAX(archived_at) FROM (
			SELECT MAX(archived_at) AS archived_at FROM archive.orders
			UNION ALL
			SELECT MAX(archived_at) FROM archive.tickets
		) sub
	`).Scan(&status.LastRunAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("last archive time: %w", err)
	}

	return status, nil
}
