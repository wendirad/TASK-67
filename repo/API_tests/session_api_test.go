//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

func TestListSessions(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/sessions")
	if resp.Code != 200 {
		t.Fatalf("List sessions failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestListProducts(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/products")
	if resp.Code != 200 {
		t.Fatalf("List products failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestAdminCreateSession(t *testing.T) {
	c := getAdminClient(t)

	// First create a facility
	facResp := c.post("/api/admin/facilities", map[string]interface{}{
		"name":         uniqueName("sess_gym"),
		"address":      "456 Test Ave",
		"checkin_mode": "staff_qr",
	})
	if facResp.Code != 201 {
		t.Fatalf("Create facility failed: %d %s", facResp.Code, facResp.Msg)
	}
	var facData struct {
		ID string `json:"id"`
	}
	json.Unmarshal(facResp.Data, &facData)

	// Create session
	resp := c.post("/api/admin/sessions", map[string]interface{}{
		"title":       "Test Yoga Class",
		"facility_id": facData.ID,
		"start_time":  "2025-06-15T10:00:00Z",
		"end_time":    "2025-06-15T11:00:00Z",
		"total_seats": 20,
		"description": "A test session",
	})
	if resp.Code != 201 {
		t.Fatalf("Create session failed: %d %s", resp.Code, resp.Msg)
	}

	var sessData struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	json.Unmarshal(resp.Data, &sessData)
	if sessData.Status != "open" {
		t.Errorf("Session status = %q, want open", sessData.Status)
	}
}

func TestSessionsRequireAuth(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/sessions")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
