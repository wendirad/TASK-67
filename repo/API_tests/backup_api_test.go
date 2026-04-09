//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBackupTriggerAndList(t *testing.T) {
	c := getAdminClient(t)

	// Trigger a backup
	resp := c.post("/api/admin/backup", nil)
	if resp.Code != 202 {
		t.Fatalf("Trigger backup failed: %d %s", resp.Code, resp.Msg)
	}

	var backup struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
		Status   string `json:"status"`
		Type     string `json:"type"`
	}
	json.Unmarshal(resp.Data, &backup)

	if backup.ID == "" {
		t.Fatal("Backup ID should not be empty")
	}
	if backup.Status != "in_progress" {
		t.Errorf("New backup status = %q, want 'in_progress'", backup.Status)
	}
	if backup.Type != "full" {
		t.Errorf("Backup type = %q, want 'full'", backup.Type)
	}

	// List backups should include the new one
	listResp := c.get("/api/admin/backups")
	if listResp.Code != 200 {
		t.Fatalf("List backups failed: %d %s", listResp.Code, listResp.Msg)
	}

	var backups []struct {
		ID string `json:"id"`
	}
	json.Unmarshal(listResp.Data, &backups)

	found := false
	for _, b := range backups {
		if b.ID == backup.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Triggered backup not found in list")
	}
}

func TestBackupTriggerRequiresAdmin(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/admin/backup", nil)
	if resp.Code != 401 {
		t.Errorf("Expected 401 for unauthenticated backup trigger, got %d", resp.Code)
	}
}

func TestBackupRestoreTargets(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/backup/restore-targets")
	if resp.Code != 200 {
		t.Fatalf("Restore targets failed: %d %s", resp.Code, resp.Msg)
	}

	var targets struct {
		BaseBackups []struct {
			ID string `json:"id"`
		} `json:"base_backups"`
	}
	json.Unmarshal(resp.Data, &targets)
	// base_backups should be an array (possibly empty)
	if targets.BaseBackups == nil {
		t.Error("base_backups should not be nil")
	}
}

func TestBackupRestoreSnapshotNotFound(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/admin/backup/restore", map[string]interface{}{
		"restore_type":       "snapshot",
		"backup_id":          "00000000-0000-0000-0000-000000000000",
		"confirmation_token": "RESTORE",
	})
	if resp.Code != 404 {
		t.Errorf("Expected 404 for nonexistent backup, got %d %s", resp.Code, resp.Msg)
	}
}

func TestBackupRestoreInvalidToken(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/admin/backup/restore", map[string]interface{}{
		"restore_type":       "snapshot",
		"backup_id":          "00000000-0000-0000-0000-000000000000",
		"confirmation_token": "WRONG",
	})
	if resp.Code != 400 {
		t.Errorf("Expected 400 for invalid confirmation token, got %d", resp.Code)
	}
}

func TestBackupRestoreInvalidType(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/admin/backup/restore", map[string]interface{}{
		"restore_type":       "invalid",
		"confirmation_token": "RESTORE",
	})
	if resp.Code != 400 {
		t.Errorf("Expected 400 for invalid restore type, got %d", resp.Code)
	}
}

func TestBackupRestorePITRFutureTime(t *testing.T) {
	c := getAdminClient(t)
	futureTime := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	resp := c.post("/api/admin/backup/restore", map[string]interface{}{
		"restore_type":       "point_in_time",
		"target_time":        futureTime,
		"confirmation_token": "RESTORE",
	})
	if resp.Code != 400 {
		t.Errorf("Expected 400 for future target time, got %d", resp.Code)
	}
}

func TestBackupRestorePITRNoBackup(t *testing.T) {
	c := getAdminClient(t)
	// Use a very old time where no backup exists
	pastTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	resp := c.post("/api/admin/backup/restore", map[string]interface{}{
		"restore_type":       "point_in_time",
		"target_time":        pastTime,
		"confirmation_token": "RESTORE",
	})
	if resp.Code != 400 {
		t.Errorf("Expected 400 for PITR with no available backup, got %d %s", resp.Code, resp.Msg)
	}
}

func TestBackupRestoreRequiresAdmin(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/admin/backup/restore", map[string]interface{}{
		"restore_type":       "snapshot",
		"backup_id":          "00000000-0000-0000-0000-000000000000",
		"confirmation_token": "RESTORE",
	})
	if resp.Code != 401 {
		t.Errorf("Expected 401 for unauthenticated restore, got %d", resp.Code)
	}
}

func TestArchiveRunAndStatus(t *testing.T) {
	c := getAdminClient(t)

	// Run archive
	resp := c.post("/api/admin/archive/run", nil)
	if resp.Code != 202 {
		t.Fatalf("Archive run failed: %d %s", resp.Code, resp.Msg)
	}

	// Check archive status
	statusResp := c.get("/api/admin/archive/status")
	if statusResp.Code != 200 {
		t.Fatalf("Archive status failed: %d %s", statusResp.Code, statusResp.Msg)
	}
}
