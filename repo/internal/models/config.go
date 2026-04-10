package models

import "time"

type ConfigEntry struct {
	ID               string    `json:"id"`
	Key              string    `json:"key"`
	Value            string    `json:"value"`
	Description      *string   `json:"description,omitempty"`
	CanaryPercentage *int      `json:"canary_percentage,omitempty"`
	UpdatedBy        *string   `json:"updated_by,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type AuditLog struct {
	ID          string    `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	Action      string    `json:"action"`
	OldValue    *string   `json:"old_value,omitempty"`
	NewValue    *string   `json:"new_value,omitempty"`
	PerformedBy *string   `json:"performed_by,omitempty"`
	IPAddress   *string   `json:"ip_address,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuditLogDMLAllowed returns whether the given DML operation is permitted on
// the audit_logs table. Only INSERT is allowed; UPDATE is always blocked;
// DELETE is only allowed after archival (enforced at DB trigger level).
func AuditLogDMLAllowed(op string) bool {
	return op == "INSERT"
}

// ArchiveAuditLogDMLAllowed returns whether the given DML operation is
// permitted on the archive.audit_logs table. Only INSERT is allowed; UPDATE
// and DELETE are always blocked (fully immutable).
func ArchiveAuditLogDMLAllowed(op string) bool {
	return op == "INSERT"
}

// AuditLogDeleteRequiresArchive returns true, reflecting the database trigger
// rule that a row must exist in archive.audit_logs before it can be deleted
// from audit_logs.
func AuditLogDeleteRequiresArchive() bool {
	return true
}
