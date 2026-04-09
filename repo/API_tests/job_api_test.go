//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

func TestJobGetRequiresAuth(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/jobs/00000000-0000-0000-0000-000000000000")
	if resp.Code != 401 {
		t.Errorf("Expected 401 for unauthenticated job access, got %d", resp.Code)
	}
}

func TestJobGetNotFound(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/jobs/00000000-0000-0000-0000-000000000000")
	if resp.Code != 404 {
		t.Errorf("Expected 404 for nonexistent job, got %d", resp.Code)
	}
}

func TestJobGetOwnershipEnforced(t *testing.T) {
	// Admin creates an export job
	admin := getAdminClient(t)
	exportResp := admin.get("/api/export?entity_type=users&format=csv")
	if exportResp.Code != 202 {
		t.Fatalf("Export job creation failed: %d %s", exportResp.Code, exportResp.Msg)
	}

	var exportData struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(exportResp.Data, &exportData)
	if exportData.JobID == "" {
		t.Fatal("Job ID should not be empty")
	}

	// Admin can view the job
	adminGet := admin.get("/api/jobs/" + exportData.JobID)
	if adminGet.Code != 200 {
		t.Errorf("Admin should be able to view own job, got %d %s", adminGet.Code, adminGet.Msg)
	}

	// A different member should NOT be able to view this job
	password := readAdminPassword()
	if password == "" {
		t.Skip("Admin password not available")
	}

	// Create a member user
	memberUsername := uniqueName("jobtest_member")
	createResp := admin.post("/api/admin/users", map[string]interface{}{
		"username":     memberUsername,
		"password":     "TestPassword123!",
		"role":         "member",
		"display_name": "Job Test Member",
	})
	if createResp.Code != 201 {
		t.Fatalf("Create member failed: %d %s", createResp.Code, createResp.Msg)
	}

	member := newClient(t)
	member.login(memberUsername, "TestPassword123!")

	// Member attempts to view admin's job
	memberGet := member.get("/api/jobs/" + exportData.JobID)
	if memberGet.Code != 403 {
		t.Errorf("Member should get 403 when viewing another user's job, got %d %s", memberGet.Code, memberGet.Msg)
	}
}
