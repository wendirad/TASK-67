package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type ConfigRepository struct {
	db *sql.DB
}

func NewConfigRepository(db *sql.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

// ListAll returns all config entries (excluding internal scheduler keys).
func (r *ConfigRepository) ListAll() ([]models.ConfigEntry, error) {
	rows, err := r.db.Query(`
		SELECT id, key, value, description, canary_percentage, updated_by, updated_at, created_at
		FROM config_entries
		WHERE key NOT LIKE 'scheduler.%'
		ORDER BY key
	`)
	if err != nil {
		return nil, fmt.Errorf("list config: %w", err)
	}
	defer rows.Close()

	var entries []models.ConfigEntry
	for rows.Next() {
		var e models.ConfigEntry
		if err := rows.Scan(&e.ID, &e.Key, &e.Value, &e.Description, &e.CanaryPercentage,
			&e.UpdatedBy, &e.UpdatedAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// FindByKey returns a config entry by key.
func (r *ConfigRepository) FindByKey(key string) (*models.ConfigEntry, error) {
	e := &models.ConfigEntry{}
	err := r.db.QueryRow(`
		SELECT id, key, value, description, canary_percentage, updated_by, updated_at, created_at
		FROM config_entries WHERE key = $1
	`, key).Scan(&e.ID, &e.Key, &e.Value, &e.Description, &e.CanaryPercentage,
		&e.UpdatedBy, &e.UpdatedAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find config: %w", err)
	}
	return e, nil
}

// Update updates a config entry's value and/or canary percentage.
func (r *ConfigRepository) Update(key, value string, canaryPercentage *int, updatedBy string) error {
	_, err := r.db.Exec(`
		UPDATE config_entries SET value = $2, canary_percentage = $3,
		    updated_by = $4, updated_at = NOW()
		WHERE key = $1
	`, key, value, canaryPercentage, updatedBy)
	if err != nil {
		return fmt.Errorf("update config: %w", err)
	}
	return nil
}

// CreateAuditLog inserts an audit log entry.
func (r *ConfigRepository) CreateAuditLog(entityType, entityID, action string, oldValue, newValue *string, performedBy, ipAddress string) error {
	_, err := r.db.Exec(`
		INSERT INTO audit_logs (entity_type, entity_id, action, old_value, new_value, performed_by, ip_address)
		VALUES ($1, $2::uuid, $3, $4, $5, $6::uuid, $7)
	`, entityType, entityID, action, oldValue, newValue, performedBy, ipAddress)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

// ListCanary returns config entries with canary percentages set.
func (r *ConfigRepository) ListCanary() ([]models.ConfigEntry, error) {
	rows, err := r.db.Query(`
		SELECT id, key, value, description, canary_percentage, updated_by, updated_at, created_at
		FROM config_entries
		WHERE canary_percentage IS NOT NULL
		ORDER BY key
	`)
	if err != nil {
		return nil, fmt.Errorf("list canary: %w", err)
	}
	defer rows.Close()

	var entries []models.ConfigEntry
	for rows.Next() {
		var e models.ConfigEntry
		if err := rows.Scan(&e.ID, &e.Key, &e.Value, &e.Description, &e.CanaryPercentage,
			&e.UpdatedBy, &e.UpdatedAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan canary config: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetIntForCohort returns the integer config value if the user's canary cohort
// falls within the canary_percentage, otherwise returns defaultVal.
// If canary_percentage is NULL the value applies to everyone.
func (r *ConfigRepository) GetIntForCohort(key string, defaultVal, userCohort int) int {
	var val string
	var canaryPct *int
	err := r.db.QueryRow(`SELECT value, canary_percentage FROM config_entries WHERE key = $1`, key).Scan(&val, &canaryPct)
	if err != nil {
		return defaultVal
	}
	n := 0
	fmt.Sscanf(val, "%d", &n)
	return models.CanaryIntValue(canaryPct, userCohort, n, defaultVal)
}

// ListAuditLogs returns recent audit logs for a given entity.
func (r *ConfigRepository) ListAuditLogs(entityType string, limit int) ([]models.AuditLog, error) {
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
