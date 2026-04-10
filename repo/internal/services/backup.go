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
	"syscall"
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
	WALArchivePath      string
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
		var err error
		if restoreType == "point_in_time" && targetTime != nil {
			err = s.executePITR(backup, *targetTime)
		} else {
			err = s.executeRestore(backup)
		}
		if err != nil {
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

	// Also take a physical base backup for PITR support
	baseDir := filepath.Join(s.cfg.BackupPath, backup.Filename+"_base")
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		log.Printf("Warning: could not create base backup dir: %v (PITR will not be available for this backup)", err)
		return nil
	}

	bbArgs := []string{
		"-h", s.cfg.DBHost,
		"-p", fmt.Sprintf("%d", s.cfg.DBPort),
		"-U", s.cfg.DBUser,
		"-D", baseDir,
		"-Ft",
		"--checkpoint=fast",
		"--wal-method=none",
	}
	bbCmd := exec.Command("pg_basebackup", bbArgs...)
	bbCmd.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.DBPassword)

	bbOutput, bbErr := bbCmd.CombinedOutput()
	if bbErr != nil {
		log.Printf("Warning: pg_basebackup failed (PITR will not be available for this backup): %v (output: %s)",
			bbErr, strings.TrimSpace(string(bbOutput)))
		os.RemoveAll(baseDir)
	} else {
		log.Printf("Physical base backup completed: %s", baseDir)
	}

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

// executePITR performs a true point-in-time recovery by restoring a physical
// base backup into a temporary PostgreSQL instance, replaying archived WAL
// segments to the target timestamp, then dumping the recovered state and
// applying it to the main database.
func (s *BackupService) executePITR(backup *models.Backup, targetTime time.Time) error {
	baseDir := filepath.Join(s.cfg.BackupPath, backup.Filename+"_base")
	baseTar := filepath.Join(baseDir, "base.tar")
	if _, err := os.Stat(baseTar); err != nil {
		return fmt.Errorf("physical base backup not found at %s — only backups taken after PITR was enabled support point-in-time recovery", baseTar)
	}

	walArchive := s.cfg.WALArchivePath
	if walArchive == "" {
		walArchive = "/wal_archive"
	}
	if entries, _ := os.ReadDir(walArchive); len(entries) == 0 {
		return fmt.Errorf("WAL archive at %s is empty — cannot replay to target time", walArchive)
	}

	// Create isolated temporary directory for recovery
	tmpDir, err := os.MkdirTemp("", "pitr-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pgData := filepath.Join(tmpDir, "pgdata")
	if err := os.MkdirAll(pgData, 0700); err != nil {
		return fmt.Errorf("create pgdata dir: %w", err)
	}

	// Extract the physical base backup
	log.Printf("PITR: extracting base backup to %s", pgData)
	tarCmd := exec.Command("tar", "xf", baseTar, "-C", pgData)
	if out, err := tarCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract base backup: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// Write pg_hba.conf for passwordless local access in the temp instance
	hbaContent := "local all all trust\nhost all all 127.0.0.1/32 trust\nhost all all ::1/128 trust\n"
	if err := os.WriteFile(filepath.Join(pgData, "pg_hba.conf"), []byte(hbaContent), 0600); err != nil {
		return fmt.Errorf("write pg_hba.conf: %w", err)
	}

	// Append recovery configuration to postgresql.auto.conf
	recoveryConf := fmt.Sprintf("\n# PITR recovery configuration\nrestore_command = 'cp %s/%%f %%p'\nrecovery_target_time = '%s'\nrecovery_target_action = 'promote'\n",
		walArchive,
		targetTime.UTC().Format("2006-01-02 15:04:05 UTC"),
	)
	confPath := filepath.Join(pgData, "postgresql.auto.conf")
	f, err := os.OpenFile(confPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open postgresql.auto.conf: %w", err)
	}
	if _, err := f.WriteString(recoveryConf); err != nil {
		f.Close()
		return fmt.Errorf("write recovery config: %w", err)
	}
	f.Close()

	// Create recovery.signal to tell PostgreSQL to enter recovery mode
	if err := os.WriteFile(filepath.Join(pgData, "recovery.signal"), nil, 0600); err != nil {
		return fmt.Errorf("create recovery.signal: %w", err)
	}

	// Fix ownership — PostgreSQL refuses to start as root
	chownCmd := exec.Command("chown", "-R", "postgres:postgres", pgData)
	if out, err := chownCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown pgdata: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// Start temporary PostgreSQL on an alternate port
	tmpPort := "5433"
	log.Printf("PITR: starting temporary PostgreSQL on port %s with recovery_target_time=%s", tmpPort, targetTime.UTC().Format(time.RFC3339))
	pgCmd := exec.Command("su-exec", "postgres",
		"postgres",
		"-D", pgData,
		"-p", tmpPort,
		"-c", "listen_addresses=127.0.0.1",
		"-c", "unix_socket_directories="+tmpDir,
		"-c", "log_min_messages=warning",
		"-c", "archive_mode=off",
	)
	pgCmd.Stdout = os.Stdout
	pgCmd.Stderr = os.Stderr

	if err := pgCmd.Start(); err != nil {
		return fmt.Errorf("start temp postgres: %w", err)
	}
	defer func() {
		pgCmd.Process.Signal(syscall.SIGTERM)
		pgCmd.Wait()
	}()

	// Wait for recovery to complete (recovery.signal is removed once done)
	log.Printf("PITR: waiting for WAL replay to complete...")
	ready := false
	for i := 0; i < 180; i++ { // 3 minute timeout
		time.Sleep(1 * time.Second)

		checkCmd := exec.Command("pg_isready", "-h", "127.0.0.1", "-p", tmpPort)
		if checkCmd.Run() != nil {
			continue
		}
		// pg_isready succeeded — verify recovery.signal is gone (recovery complete)
		if _, err := os.Stat(filepath.Join(pgData, "recovery.signal")); os.IsNotExist(err) {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("temporary PostgreSQL did not complete WAL recovery within 3 minutes")
	}
	log.Printf("PITR: WAL replay complete, dumping recovered state")

	// Dump the recovered state from the temp instance
	recoveredDump := filepath.Join(tmpDir, "recovered.dump")
	dumpArgs := []string{
		"-Fc",
		"-h", "127.0.0.1",
		"-p", tmpPort,
		"-U", s.cfg.DBUser,
		"-d", s.cfg.DBName,
		"-f", recoveredDump,
	}
	dumpCmd := exec.Command("pg_dump", dumpArgs...)
	if out, err := dumpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dump recovered state: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// Apply the recovered dump to the main database
	log.Printf("PITR: applying recovered state to main database")
	restoreArgs := []string{
		"-h", s.cfg.DBHost,
		"-p", fmt.Sprintf("%d", s.cfg.DBPort),
		"-U", s.cfg.DBUser,
		"-d", s.cfg.DBName,
		"--clean",
		"--if-exists",
		recoveredDump,
	}
	restoreCmd := exec.Command("pg_restore", restoreArgs...)
	restoreCmd.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.DBPassword)

	if out, err := restoreCmd.CombinedOutput(); err != nil {
		outStr := strings.TrimSpace(string(out))
		if strings.Contains(outStr, "ERROR") {
			return fmt.Errorf("apply recovered state: %w (%s)", err, outStr)
		}
		log.Printf("PITR restore completed with warnings: %s", outStr)
	}

	log.Printf("PITR completed successfully: target=%s base_backup=%s", targetTime.Format(time.RFC3339), backup.Filename)
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
