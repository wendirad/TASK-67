package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

// BackupConfig holds the configuration needed for backup/restore execution.
type BackupConfig struct {
	DBHost              string
	DBPort              int
	DBName              string
	DBUser              string
	DBPassword          string
	BackupPath          string
	BackupEncryptionKey string
}

type BackupService struct {
	backupRepo *repository.BackupRepository
	cfg        BackupConfig
}

func NewBackupService(backupRepo *repository.BackupRepository, cfg BackupConfig) *BackupService {
	return &BackupService{backupRepo: backupRepo, cfg: cfg}
}

// TriggerBackup creates a backup record and returns it.
// The actual backup execution is handled by the worker.
func (s *BackupService) TriggerBackup() (*models.Backup, int, string) {
	filename := "base_" + time.Now().Format("20060102_150405")
	encrypted := s.cfg.BackupEncryptionKey != ""

	backup, err := s.backupRepo.CreateBackup(filename, "full", encrypted)
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

// TriggerRestore validates restore parameters and executes the restore.
func (s *BackupService) TriggerRestore(restoreType, backupID, confirmationToken string, targetTime *time.Time) (*models.Backup, int, string) {
	if confirmationToken != "RESTORE" {
		return nil, 400, "Confirmation token must be the literal string RESTORE"
	}

	if restoreType != "snapshot" && restoreType != "point_in_time" {
		return nil, 400, "restore_type must be 'snapshot' or 'point_in_time'"
	}

	var backup *models.Backup

	if restoreType == "snapshot" {
		if backupID == "" {
			return nil, 400, "backup_id is required for snapshot restore"
		}

		b, err := s.backupRepo.FindByID(backupID)
		if err != nil {
			log.Printf("Error finding backup %s: %v", backupID, err)
			return nil, 500, "Internal server error"
		}
		if b == nil {
			return nil, 404, "Backup not found"
		}
		if b.Status != "completed" {
			return nil, 400, "Can only restore from a completed backup"
		}
		backup = b
	} else {
		// Point-in-time restore
		if targetTime == nil {
			return nil, 400, "target_time is required for point_in_time restore"
		}

		if targetTime.After(time.Now()) {
			return nil, 400, "target_time cannot be in the future"
		}

		baseBackup, err := s.backupRepo.FindBaseBackupForPITR(*targetTime)
		if err != nil {
			log.Printf("Error finding base backup for PITR: %v", err)
			return nil, 500, "Internal server error"
		}
		if baseBackup == nil {
			return nil, 400, "No completed backup available before the target time"
		}
		backup = baseBackup
	}

	// Execute restore asynchronously
	go func() {
		if err := s.executeRestore(backup); err != nil {
			log.Printf("Restore failed for backup %s: %v", backup.ID, err)
		} else {
			log.Printf("Restore completed successfully from backup %s (%s)", backup.ID, backup.Filename)
		}
	}()

	log.Printf("Restore job accepted: type=%s backup=%s", restoreType, backup.ID)
	return backup, 202, "Restore job accepted"
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

// ExecuteBackup runs pg_dump for a given backup record and updates the record on completion.
func (s *BackupService) ExecuteBackup(backup *models.Backup) error {
	if err := os.MkdirAll(s.cfg.BackupPath, 0700); err != nil {
		s.backupRepo.FailBackup(backup.ID)
		return fmt.Errorf("create backup directory: %w", err)
	}

	dumpFile := filepath.Join(s.cfg.BackupPath, backup.Filename+".dump")

	// Capture current WAL LSN before backup
	walLSN := s.getCurrentWALLSN()

	// Execute pg_dump in custom format
	args := []string{
		"-Fc",
		"-h", s.cfg.DBHost,
		"-p", fmt.Sprintf("%d", s.cfg.DBPort),
		"-U", s.cfg.DBUser,
		"-d", s.cfg.DBName,
		"-f", dumpFile,
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.DBPassword)

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.backupRepo.FailBackup(backup.ID)
		return fmt.Errorf("pg_dump failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	// Encrypt if configured
	finalFile := dumpFile
	if backup.Encrypted && s.cfg.BackupEncryptionKey != "" {
		encFile := dumpFile + ".enc"
		if err := EncryptFile(dumpFile, encFile, s.cfg.BackupEncryptionKey); err != nil {
			s.backupRepo.FailBackup(backup.ID)
			return fmt.Errorf("encrypt backup: %w", err)
		}
		os.Remove(dumpFile)
		finalFile = encFile
	}

	// Get file size
	info, err := os.Stat(finalFile)
	if err != nil {
		s.backupRepo.FailBackup(backup.ID)
		return fmt.Errorf("stat backup file: %w", err)
	}

	if err := s.backupRepo.CompleteBackup(backup.ID, info.Size(), walLSN); err != nil {
		return fmt.Errorf("complete backup record: %w", err)
	}

	log.Printf("Backup executed: id=%s file=%s size=%d wal_lsn=%s", backup.ID, finalFile, info.Size(), walLSN)
	return nil
}

// executeRestore runs pg_restore from a backup file.
func (s *BackupService) executeRestore(backup *models.Backup) error {
	dumpFile := filepath.Join(s.cfg.BackupPath, backup.Filename+".dump")
	encFile := dumpFile + ".enc"

	// Determine the actual file to restore from
	restoreFile := dumpFile
	if backup.Encrypted {
		if s.cfg.BackupEncryptionKey == "" {
			return fmt.Errorf("backup is encrypted but no encryption key configured")
		}
		// Check if encrypted file exists
		if _, err := os.Stat(encFile); err == nil {
			// Decrypt to temporary file
			tmpFile := dumpFile + ".tmp"
			if err := DecryptFile(encFile, tmpFile, s.cfg.BackupEncryptionKey); err != nil {
				return fmt.Errorf("decrypt backup: %w", err)
			}
			defer os.Remove(tmpFile)
			restoreFile = tmpFile
		} else if _, err := os.Stat(dumpFile); err != nil {
			return fmt.Errorf("backup file not found: %s", backup.Filename)
		}
	} else {
		if _, err := os.Stat(dumpFile); err != nil {
			return fmt.Errorf("backup file not found: %s", dumpFile)
		}
	}

	// Execute pg_restore with --clean to drop and recreate objects
	args := []string{
		"-h", s.cfg.DBHost,
		"-p", fmt.Sprintf("%d", s.cfg.DBPort),
		"-U", s.cfg.DBUser,
		"-d", s.cfg.DBName,
		"--clean",
		"--if-exists",
		restoreFile,
	}

	cmd := exec.Command("pg_restore", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.DBPassword)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// pg_restore returns non-zero even for warnings; check output
		outStr := strings.TrimSpace(string(output))
		if strings.Contains(outStr, "ERROR") {
			return fmt.Errorf("pg_restore failed: %w (output: %s)", err, outStr)
		}
		log.Printf("pg_restore completed with warnings: %s", outStr)
	}

	log.Printf("Restore completed: backup=%s file=%s", backup.ID, restoreFile)
	return nil
}

// getCurrentWALLSN queries the current WAL log sequence number.
func (s *BackupService) getCurrentWALLSN() string {
	// This requires a DB connection; use the repo's underlying connection
	lsn, err := s.backupRepo.GetCurrentWALLSN()
	if err != nil {
		log.Printf("Warning: could not get current WAL LSN: %v", err)
		return ""
	}
	return lsn
}

// EncryptFile encrypts src file to dst using AES-256-GCM with the given key.
func EncryptFile(src, dst, key string) error {
	plaintext, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	encrypted, err := AESGCMEncrypt(plaintext, key)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, encrypted, 0600); err != nil {
		return fmt.Errorf("write encrypted: %w", err)
	}
	return nil
}

// DecryptFile decrypts src file to dst using AES-256-GCM with the given key.
func DecryptFile(src, dst, key string) error {
	ciphertext, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read encrypted: %w", err)
	}

	plaintext, err := AESGCMDecrypt(ciphertext, key)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, plaintext, 0600); err != nil {
		return fmt.Errorf("write decrypted: %w", err)
	}
	return nil
}

// AESGCMEncrypt encrypts data using AES-256-GCM. Key is SHA-256 hashed to ensure 32 bytes.
// Output format: [12-byte nonce][ciphertext+tag]
func AESGCMEncrypt(plaintext []byte, key string) ([]byte, error) {
	keyHash := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// AESGCMDecrypt decrypts data encrypted with AESGCMEncrypt.
func AESGCMDecrypt(ciphertext []byte, key string) ([]byte, error) {
	keyHash := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBody := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBody, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
