package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

// CreateJob inserts a new job.
func (r *JobRepository) CreateJob(job *models.Job) error {
	return r.db.QueryRow(`
		INSERT INTO jobs (type, payload, status, scheduled_at)
		VALUES ($1, $2, 'pending', NOW())
		RETURNING id, status, attempts, max_attempts, scheduled_at, created_at
	`, job.Type, job.Payload).Scan(
		&job.ID, &job.Status, &job.Attempts, &job.MaxAttempts,
		&job.ScheduledAt, &job.CreatedAt,
	)
}

// FindByID returns a job by ID.
func (r *JobRepository) FindByID(id string) (*models.Job, error) {
	j := &models.Job{}
	err := r.db.QueryRow(`
		SELECT id, type, status, payload, result, attempts, max_attempts,
		       scheduled_at, started_at, completed_at, created_at
		FROM jobs WHERE id = $1
	`, id).Scan(
		&j.ID, &j.Type, &j.Status, &j.Payload, &j.Result,
		&j.Attempts, &j.MaxAttempts, &j.ScheduledAt, &j.StartedAt,
		&j.CompletedAt, &j.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find job: %w", err)
	}
	return j, nil
}

// ClaimPendingJobs atomically selects and transitions up to N pending jobs to
// 'processing' in a single statement. The CTE uses FOR UPDATE SKIP LOCKED to
// prevent duplicate pickup under concurrent workers.
func (r *JobRepository) ClaimPendingJobs(limit int) ([]models.Job, error) {
	rows, err := r.db.Query(`
		WITH picked AS (
			SELECT id FROM jobs
			WHERE status = 'pending' AND scheduled_at <= NOW()
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE jobs SET status = 'processing', started_at = NOW(), attempts = attempts + 1
		FROM picked WHERE jobs.id = picked.id
		RETURNING jobs.id, jobs.type, jobs.status, jobs.payload, jobs.result,
		          jobs.attempts, jobs.max_attempts, jobs.scheduled_at,
		          jobs.started_at, jobs.completed_at, jobs.created_at
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		if err := rows.Scan(
			&j.ID, &j.Type, &j.Status, &j.Payload, &j.Result,
			&j.Attempts, &j.MaxAttempts, &j.ScheduledAt, &j.StartedAt,
			&j.CompletedAt, &j.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// CompleteJob marks a job as completed with result.
func (r *JobRepository) CompleteJob(jobID, result string) error {
	_, err := r.db.Exec(`
		UPDATE jobs SET status = 'completed', result = $2, completed_at = NOW()
		WHERE id = $1
	`, jobID, result)
	return err
}

// FailJob marks a job as failed or resets to pending for retry.
func (r *JobRepository) FailJob(jobID string, errMsg string) error {
	_, err := r.db.Exec(`
		UPDATE jobs SET
		    status = CASE WHEN attempts < max_attempts THEN 'pending' ELSE 'failed' END,
		    result = $2,
		    scheduled_at = CASE WHEN attempts < max_attempts THEN NOW() + INTERVAL '30 seconds' * attempts ELSE scheduled_at END
		WHERE id = $1
	`, jobID, errMsg)
	return err
}

// CreateFileRecord inserts a file_records entry.
func (r *JobRepository) CreateFileRecord(fr *models.FileRecord) error {
	return r.db.QueryRow(`
		INSERT INTO file_records (filename, file_type, sha256_hash, size_bytes, uploaded_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, fr.Filename, fr.FileType, fr.SHA256Hash, fr.SizeBytes, fr.UploadedBy).Scan(&fr.ID, &fr.CreatedAt)
}

// CheckDuplicateFile checks if a file with the same hash already exists.
func (r *JobRepository) CheckDuplicateFile(hash string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM file_records WHERE sha256_hash = $1)
	`, hash).Scan(&exists)
	return exists, err
}

// DB returns the underlying database handle.
func (r *JobRepository) DB() *sql.DB {
	return r.db
}
