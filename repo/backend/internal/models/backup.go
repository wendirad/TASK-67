package models

import "time"

type Backup struct {
	ID          string     `json:"id"`
	Filename    string     `json:"filename"`
	SizeBytes   int64      `json:"size_bytes"`
	Encrypted   bool       `json:"encrypted"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	WALStartLSN *string    `json:"wal_start_lsn,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type RestoreTargets struct {
	EarliestSnapshot *time.Time     `json:"earliest_snapshot"`
	EarliestPITR     *time.Time     `json:"earliest_pitr"`
	LatestPITR       *time.Time     `json:"latest_pitr"`
	BaseBackups      []BackupSummary `json:"base_backups"`
}

type BackupSummary struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	SizeBytes int64     `json:"size_bytes"`
}

type ArchiveStatus struct {
	LastRunAt      *time.Time `json:"last_run_at"`
	OrdersArchived int        `json:"orders_archived"`
	TicketsArchived int       `json:"tickets_archived"`
	AuditLogsArchived int     `json:"audit_logs_archived"`
	TotalArchived  int        `json:"total_archived"`
}
