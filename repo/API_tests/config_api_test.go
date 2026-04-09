//go:build integration

package api_tests

import (
	"testing"
)

func TestConfigList(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/config")
	if resp.Code != 200 {
		t.Fatalf("Config list failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestConfigCanaryList(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/config-canary")
	if resp.Code != 200 {
		t.Fatalf("Canary list failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestConfigAuditLogs(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/admin/config-audit-logs")
	if resp.Code != 200 {
		t.Fatalf("Audit logs failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestConfigUpdateNonexistent(t *testing.T) {
	c := getAdminClient(t)
	resp := c.put("/api/admin/config/nonexistent_key", map[string]interface{}{
		"value": "test",
	})
	if resp.Code == 200 {
		t.Fatal("Expected update of nonexistent key to fail")
	}
}

func TestConfigRequiresAdmin(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/admin/config")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
