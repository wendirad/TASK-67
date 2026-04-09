package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"campusrec/internal/config"
	"campusrec/internal/database"
	"campusrec/internal/worker"
	"campusrec/internal/worker/jobs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := worker.NewScheduler(db)

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "waitlist_promoter",
		LockID:   jobs.WaitlistPromoterLockID,
		Interval: 10 * time.Second,
		Fn:       jobs.WaitlistPromoter(db),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "order_closer",
		LockID:   jobs.OrderCloserLockID,
		Interval: 30 * time.Second,
		Fn:       jobs.OrderCloser(db),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "noshow_detector",
		LockID:   jobs.NoShowDetectorLockID,
		Interval: 30 * time.Second,
		Fn:       jobs.NoShowDetector(db),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "break_overrun_detector",
		LockID:   jobs.BreakOverrunDetectorLockID,
		Interval: 15 * time.Second,
		Fn:       jobs.BreakOverrunDetector(db),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "session_status_updater",
		LockID:   jobs.SessionStatusUpdaterLockID,
		Interval: 60 * time.Second,
		Fn:       jobs.SessionStatusUpdater(db),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "sla_checker",
		LockID:   jobs.SLACheckerLockID,
		Interval: 5 * time.Minute,
		Fn:       jobs.SLAChecker(db),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "job_processor",
		LockID:   jobs.JobProcessorLockID,
		Interval: 5 * time.Second,
		Fn:       jobs.JobProcessor(db),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "archiver",
		LockID:   jobs.ArchiverLockID,
		Interval: 24 * time.Hour,
		Fn:       jobs.Archiver(db),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "backup_executor",
		LockID:   jobs.BackupExecutorLockID,
		Interval: 10 * time.Second,
		Fn:       jobs.BackupExecutor(db, cfg),
	})

	scheduler.AddJob(worker.ScheduledJob{
		Name:     "daily_backup",
		LockID:   jobs.DailyBackupLockID,
		Interval: 24 * time.Hour,
		Fn:       jobs.DailyBackup(db, cfg),
	})

	log.Println("Worker started")

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Worker received shutdown signal")
		cancel()
	}()

	scheduler.Run(ctx)
	log.Println("Worker shut down")
}
