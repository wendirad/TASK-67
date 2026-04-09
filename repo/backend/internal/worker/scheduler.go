package worker

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// ScheduledJob defines a recurring background job.
type ScheduledJob struct {
	Name     string
	LockID   int64
	Interval time.Duration
	Fn       func(ctx context.Context) error
}

// Scheduler manages background jobs with leader election and per-job advisory locks.
type Scheduler struct {
	jobs []ScheduledJob
	db   *sql.DB
}

// NewScheduler creates a new scheduler.
func NewScheduler(db *sql.DB) *Scheduler {
	return &Scheduler{db: db}
}

// AddJob registers a scheduled job.
func (s *Scheduler) AddJob(job ScheduledJob) {
	s.jobs = append(s.jobs, job)
}

// Run starts the scheduler. It blocks until ctx is canceled.
// Acquires leader lock first, then runs jobs in a loop.
func (s *Scheduler) Run(ctx context.Context) {
	s.waitForLeadership(ctx)
	if ctx.Err() != nil {
		return
	}

	log.Println("Scheduler: acquired leader lock, starting job loop")

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler: shutting down")
			return
		default:
			for i := range s.jobs {
				if ctx.Err() != nil {
					return
				}
				lastRun := s.getLastRun(ctx, s.jobs[i].Name)
				if time.Since(lastRun) >= s.jobs[i].Interval {
					if err := s.runJob(ctx, s.jobs[i]); err == nil {
						s.setLastRun(ctx, s.jobs[i].Name, time.Now())
					}
				}
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// waitForLeadership blocks until the leader advisory lock is acquired.
func (s *Scheduler) waitForLeadership(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var acquired bool
			err := s.db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(1)").Scan(&acquired)
			if err != nil {
				log.Printf("Scheduler: error acquiring leader lock: %v", err)
				time.Sleep(30 * time.Second)
				continue
			}
			if acquired {
				return
			}
			log.Println("Scheduler: leader lock held by another instance, retrying in 30s")
			time.Sleep(30 * time.Second)
		}
	}
}

// runJob executes a single job with a per-job advisory lock.
func (s *Scheduler) runJob(ctx context.Context, job ScheduledJob) error {
	var acquired bool
	err := s.db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", job.LockID).Scan(&acquired)
	if err != nil {
		log.Printf("Scheduler: error acquiring lock for job %s: %v", job.Name, err)
		return err
	}
	if !acquired {
		return nil // Another execution in progress, skip
	}
	defer func() {
		_, _ = s.db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", job.LockID)
	}()

	if err := job.Fn(ctx); err != nil {
		log.Printf("Scheduler: job %s failed: %v", job.Name, err)
		return err
	}
	return nil
}

// getLastRun retrieves the last run time for a job from config_entries.
func (s *Scheduler) getLastRun(ctx context.Context, jobName string) time.Time {
	key := "scheduler.last_run." + jobName
	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM config_entries WHERE key = $1`, key,
	).Scan(&value)
	if err != nil {
		return time.Time{} // Never run
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

// setLastRun stores the last run time for a job in config_entries.
func (s *Scheduler) setLastRun(ctx context.Context, jobName string, t time.Time) {
	key := "scheduler.last_run." + jobName
	value := t.Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO config_entries (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()
	`, key, value)
	if err != nil {
		log.Printf("Scheduler: error saving last run for %s: %v", jobName, err)
	}
}
