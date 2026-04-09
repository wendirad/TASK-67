package models

import "time"

type Job struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Payload     *string    `json:"payload,omitempty"`
	Result      *string    `json:"result,omitempty"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type FileRecord struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	FileType   string    `json:"file_type"`
	SHA256Hash string    `json:"sha256_hash"`
	SizeBytes  int64     `json:"size_bytes"`
	UploadedBy *string   `json:"uploaded_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
