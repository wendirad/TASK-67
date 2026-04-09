package services

import (
	"log"
	"time"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type BackupService struct {
	backupRepo *repository.BackupRepository
}

func NewBackupService(backupRepo *repository.BackupRepository) *BackupService {
	return &BackupService{backupRepo: backupRepo}
}

// TriggerBackup creates a backup record and returns it.
// The actual backup execution is handled by the worker.
func (s *BackupService) TriggerBackup() (*models.Backup, int, string) {
	filename := "base_" + time.Now().Format("20060102_150405")

	backup, err := s.backupRepo.CreateBackup(filename, "full")
	if err != nil {
		log.Printf("Error creating backup record: %v", err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Backup triggered: %s (id=%s)", filename, backup.ID)
	return backup, 202, "Backup job started"
}

// ListBackups returns all backup records.
func (s *BackupService) ListBackups() ([]models.Backup, error) {
	backups, err := s.backupRepo.ListBackups()
	if err != nil {
		return nil, err
	}
	if backups == nil {
		backups = []models.Backup{}
	}
	return backups, nil
}

// GetRestoreTargets returns the available restore window.
func (s *BackupService) GetRestoreTargets() (*models.RestoreTargets, error) {
	return s.backupRepo.GetRestoreTargets()
}

// TriggerRestore validates restore parameters and creates a restore job record.
func (s *BackupService) TriggerRestore(restoreType, backupID, confirmationToken string, targetTime *time.Time) (*models.Backup, int, string) {
	if confirmationToken != "RESTORE" {
		return nil, 400, "Confirmation token must be the literal string RESTORE"
	}

	if restoreType != "snapshot" && restoreType != "point_in_time" {
		return nil, 400, "restore_type must be 'snapshot' or 'point_in_time'"
	}

	if restoreType == "snapshot" {
		if backupID == "" {
			return nil, 400, "backup_id is required for snapshot restore"
		}

		backup, err := s.backupRepo.FindByID(backupID)
		if err != nil {
			log.Printf("Error finding backup %s: %v", backupID, err)
			return nil, 500, "Internal server error"
		}
		if backup == nil {
			return nil, 404, "Backup not found"
		}
		if backup.Status != "completed" {
			return nil, 400, "Can only restore from a completed backup"
		}

		log.Printf("Snapshot restore requested for backup %s", backupID)
		return backup, 202, "Restore job accepted"
	}

	// Point-in-time restore
	if targetTime == nil {
		return nil, 400, "target_time is required for point_in_time restore"
	}

	if targetTime.After(time.Now()) {
		return nil, 400, "target_time cannot be in the future"
	}

	// Find the base backup for the target time
	baseBackup, err := s.backupRepo.FindBaseBackupForPITR(*targetTime)
	if err != nil {
		log.Printf("Error finding base backup for PITR: %v", err)
		return nil, 500, "Internal server error"
	}
	if baseBackup == nil {
		return nil, 400, "No completed backup available before the target time"
	}

	log.Printf("PITR restore requested: target=%s, base_backup=%s", targetTime.Format(time.RFC3339), baseBackup.ID)
	return baseBackup, 202, "Restore job accepted"
}

// RunArchive triggers the archive process for orders and tickets.
func (s *BackupService) RunArchive() (*models.ArchiveStatus, int, string) {
	ordersArchived, err := s.backupRepo.ArchiveOrders(24, 500)
	if err != nil {
		log.Printf("Error archiving orders: %v", err)
		return nil, 500, "Failed to archive orders"
	}

	ticketsArchived, err := s.backupRepo.ArchiveTickets(24, 500)
	if err != nil {
		log.Printf("Error archiving tickets: %v", err)
		return nil, 500, "Failed to archive tickets"
	}

	log.Printf("Archive completed: %d orders, %d tickets archived", ordersArchived, ticketsArchived)

	status, err := s.backupRepo.GetArchiveStatus()
	if err != nil {
		log.Printf("Error getting archive status: %v", err)
		return nil, 500, "Archive completed but failed to get status"
	}

	return status, 202, "Archive job completed"
}

// GetArchiveStatus returns the current archive status.
func (s *BackupService) GetArchiveStatus() (*models.ArchiveStatus, error) {
	return s.backupRepo.GetArchiveStatus()
}
