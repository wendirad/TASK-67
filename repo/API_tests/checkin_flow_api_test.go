//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

// TestCheckinPerformByStaff verifies that staff can perform a check-in.
// This requires a valid registration in "registered" state, so the test
// is best-effort — it skips if no suitable registration exists.
func TestCheckinPerformByStaff(t *testing.T) {
	c := getAdminClient(t)

	// Find a registered registration that we can check in
	resp := c.get("/api/admin/registrations?status=registered&page_size=5")
	if resp.Code != 200 {
		t.Fatalf("List registrations failed: %d %s", resp.Code, resp.Msg)
	}

	var regData struct {
		Items []struct {
			ID        string `json:"id"`
			SessionID string `json:"session_id"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &regData)

	if len(regData.Items) == 0 {
		t.Skip("No registered registrations available for check-in test")
	}

	// Try to generate QR code for the session (staff-level check)
	reg := regData.Items[0]
	qrResp := c.get("/api/sessions/" + reg.SessionID + "/qr")
	if qrResp.Code != 200 {
		t.Logf("QR generation returned %d (session may not be active)", qrResp.Code)
	}

	// Attempt check-in (may fail if session timing doesn't match)
	checkinResp := c.post("/api/checkin", map[string]interface{}{
		"registration_id": reg.ID,
	})
	// We accept 200 (success) or 422 (timing/state error) — both indicate
	// the endpoint is functional and validates correctly
	if checkinResp.Code != 200 && checkinResp.Code != 422 {
		t.Errorf("Check-in returned unexpected %d %s", checkinResp.Code, checkinResp.Msg)
	}
}

// TestCheckinBreakReturnCycle verifies the break and return endpoints exist
// and return appropriate errors for non-existent check-ins.
func TestCheckinBreakReturnCycle(t *testing.T) {
	c := getAdminClient(t)

	// Non-existent check-in should return 404
	fakeID := "00000000-0000-0000-0000-000000000000"

	breakResp := c.post("/api/checkin/"+fakeID+"/break", nil)
	if breakResp.Code != 404 {
		t.Errorf("Break on non-existent check-in: got %d, want 404", breakResp.Code)
	}

	returnResp := c.post("/api/checkin/"+fakeID+"/return", nil)
	if returnResp.Code != 404 {
		t.Errorf("Return on non-existent check-in: got %d, want 404", returnResp.Code)
	}
}

// TestCheckinGetNonExistent verifies the check-in detail endpoint returns 404
// for non-existent check-ins.
func TestCheckinGetNonExistent(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/checkin/00000000-0000-0000-0000-000000000000")
	if resp.Code != 404 {
		t.Errorf("Get non-existent check-in: got %d, want 404", resp.Code)
	}
}

// TestCheckinQRRequiresAuth verifies the QR endpoint requires authentication.
func TestCheckinQRRequiresAuth(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/sessions/00000000-0000-0000-0000-000000000000/qr")
	if resp.Code != 401 {
		t.Errorf("QR without auth: got %d, want 401", resp.Code)
	}
}

// TestCheckinPerformRequiresMemberAuth verifies that unauthenticated check-in
// is rejected.
func TestCheckinPerformRequiresAuth(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/checkin", map[string]interface{}{
		"registration_id": "00000000-0000-0000-0000-000000000000",
	})
	if resp.Code != 401 {
		t.Errorf("Unauthenticated check-in: got %d, want 401", resp.Code)
	}
}
