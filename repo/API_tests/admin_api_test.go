//go:build integration

package api_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func getAdminClient(t *testing.T) *apiClient {
	t.Helper()
	password := readAdminPassword()
	if password == "" {
		t.Skip("Admin password not available")
	}
	c := newClient(t)
	c.login("admin", password)
	return c
}

func TestAdminListUsers(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/users")
	if resp.Code != 200 {
		t.Fatalf("List users failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Items []struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"items"`
		Total int `json:"total"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.Total < 1 {
		t.Error("Expected at least 1 user (admin)")
	}
}

func TestAdminCreateUser(t *testing.T) {
	c := getAdminClient(t)
	username := fmt.Sprintf("testmember_%d", os.Getpid())
	resp := c.post("/api/admin/users", map[string]interface{}{
		"username":     username,
		"password":     "TestPassword123!",
		"role":         "member",
		"display_name": "Test Member",
	})
	if resp.Code != 201 {
		t.Fatalf("Create user failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminCreateUserDuplicate(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/admin/users", map[string]interface{}{
		"username":     "admin",
		"password":     "TestPassword123!",
		"role":         "member",
		"display_name": "Duplicate",
	})
	if resp.Code == 201 {
		t.Fatal("Expected duplicate username to fail")
	}
}

func TestAdminListFacilities(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/facilities")
	if resp.Code != 200 {
		t.Fatalf("List facilities failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminCreateFacility(t *testing.T) {
	c := getAdminClient(t)
	name := fmt.Sprintf("Test Gym %d", os.Getpid())
	resp := c.post("/api/admin/facilities", map[string]interface{}{
		"name":         name,
		"address":      "123 Test St",
		"checkin_mode": "staff_qr",
	})
	if resp.Code != 201 {
		t.Fatalf("Create facility failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminListConfig(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/config")
	if resp.Code != 200 {
		t.Fatalf("List config failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminListCanary(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/config-canary")
	if resp.Code != 200 {
		t.Fatalf("List canary failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminListAuditLogs(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/config-audit-logs")
	if resp.Code != 200 {
		t.Fatalf("List audit logs failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminListBackups(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/backups")
	if resp.Code != 200 {
		t.Fatalf("List backups failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminRestoreTargets(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/backup/restore-targets")
	if resp.Code != 200 {
		t.Fatalf("Restore targets failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminArchiveStatus(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/archive/status")
	if resp.Code != 200 {
		t.Fatalf("Archive status failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminRestoreRequiresConfirmation(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/admin/backup/restore", map[string]interface{}{
		"restore_type":       "snapshot",
		"backup_id":          "00000000-0000-0000-0000-000000000000",
		"confirmation_token": "WRONG",
	})
	if resp.Code == 202 {
		t.Fatal("Expected restore to fail without correct confirmation token")
	}
}
