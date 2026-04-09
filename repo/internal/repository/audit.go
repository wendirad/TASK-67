package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

// AuditRepository handles audit log persistence.
type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Log creates an audit log entry. performedBy may be empty for system-initiated actions,
// in which case it is stored as NULL.
func (r *AuditRepository) Log(entityType, entityID, action string, oldValue, newValue *string, performedBy, ipAddress string) error {
	var performedByParam interface{}
	if performedBy != "" {
		performedByParam = performedBy
	}

	_, err := r.db.Exec(`
		INSERT INTO audit_logs (entity_type, entity_id, action, old_value, new_value, performed_by, ip_address)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, $7)
	`, entityType, entityID, action, oldValue, newValue, performedByParam, ipAddress)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

// ListByEntityType returns recent audit logs filtered by entity type.
func (r *AuditRepository) ListByEntityType(entityType string, limit int) ([]models.AuditLog, error) {
	rows, err := r.db.Query(`
		SELECT id, entity_type, entity_id::text, action, old_value, new_value,
		       performed_by::text, ip_address, created_at
		FROM audit_logs
		WHERE entity_type = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, entityType, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID, &l.Action,
			&l.OldValue, &l.NewValue, &l.PerformedBy, &l.IPAddress, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
