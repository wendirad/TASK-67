package jobs

import (
	"context"
	"database/sql"
	"log"
	"time"

	"campusrec/internal/config"
	"campusrec/internal/repository"
	"campusrec/internal/services"
)

const BackupExecutorLockID int64 = 108
const DailyBackupLockID int64 = 109

// BackupExecutor finds in_progress backup records and executes pg_dump for each.
func BackupExecutor(db *sql.DB, cfg *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		backupRepo := repository.NewBackupRepository(db)
		backupSvc := services.NewBackupService(backupRepo, services.BackupConfig{
			DBHost:              cfg.DBHost,
			DBPort:              cfg.DBPort,
			DBName:              cfg.DBName,
			DBUser:              cfg.DBUser,
			DBPassword:          cfg.DBPassword,
			BackupPath:          cfg.BackupPath,
			BackupEncryptionKey: cfg.BackupEncryptionKey,
			WALArchivePath:      cfg.WALArchivePath,
		})

		pending, err := backupRepo.FindPendingBackups()
		if err != nil {
			log.Printf("BackupExecutor: error finding pending backups: %v", err)
			return err
		}

		for _, backup := range pending {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			log.Printf("BackupExecutor: executing backup %s (%s)", backup.ID, backup.Filename)
			if err := backupSvc.ExecuteBackup(&backup); err != nil {
				log.Printf("BackupExecutor: backup %s failed: %v", backup.ID, err)
				continue
			}
			log.Printf("BackupExecutor: backup %s completed", backup.ID)
		}

		return nil
	}
}

// DailyBackup creates a scheduled daily backup record and executes it.
func DailyBackup(db *sql.DB, cfg *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		backupRepo := repository.NewBackupRepository(db)
		backupSvc := services.NewBackupService(backupRepo, services.BackupConfig{
			DBHost:              cfg.DBHost,
			DBPort:              cfg.DBPort,
			DBName:              cfg.DBName,
			DBUser:              cfg.DBUser,
			DBPassword:          cfg.DBPassword,
			BackupPath:          cfg.BackupPath,
			BackupEncryptionKey: cfg.BackupEncryptionKey,
			WALArchivePath:      cfg.WALArchivePath,
		})

		encrypted := cfg.BackupEncryptionKey != ""
		filename := "daily_" + time.Now().Format("20060102_150405")

		backup, err := backupRepo.CreateBackup(filename, "full", encrypted)
		if err != nil {
			log.Printf("DailyBackup: error creating backup record: %v", err)
			return err
		}

		log.Printf("DailyBackup: executing backup %s (%s)", backup.ID, backup.Filename)
		if err := backupSvc.ExecuteBackup(backup); err != nil {
			log.Printf("DailyBackup: backup %s failed: %v", backup.ID, err)
			return err
		}

		// Clean up old backups (retain 7 days)
		olderThan := time.Now().AddDate(0, 0, -7)
		deleted, err := backupRepo.DeleteOldBackups(olderThan)
		if err != nil {
			log.Printf("DailyBackup: error cleaning old backups: %v", err)
		} else if deleted > 0 {
			log.Printf("DailyBackup: cleaned %d old backup records", deleted)
		}

		log.Printf("DailyBackup: completed successfully")
		return nil
	}
}
