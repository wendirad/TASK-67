//go:build integration

package api_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// createOpenSession creates a facility and an open session with the given seat
// count. Returns (facilityID, sessionID).
func createOpenSession(t *testing.T, c *apiClient, totalSeats int) (string, string) {
	t.Helper()

	facResp := c.post("/api/admin/facilities", map[string]interface{}{
		"name":         fmt.Sprintf("reg_fac_%d_%d", os.Getpid(), time.Now().UnixMilli()),
		"address":      "100 Test Lane",
		"checkin_mode": "staff_qr",
	})
	if facResp.Code != 201 {
		t.Fatalf("Create facility failed: %d %s", facResp.Code, facResp.Msg)
	}
	var fac struct {
		ID string `json:"id"`
	}
	json.Unmarshal(facResp.Data, &fac)

	start := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	end := time.Now().Add(49 * time.Hour).UTC().Format(time.RFC3339)

	sessResp := c.post("/api/admin/sessions", map[string]interface{}{
		"title":       fmt.Sprintf("RegTest Session %d", time.Now().UnixMilli()),
		"facility_id": fac.ID,
		"start_time":  start,
		"end_time":    end,
		"total_seats": totalSeats,
		"description": "Registration test session",
	})
	if sessResp.Code != 201 {
		t.Fatalf("Create session failed: %d %s", sessResp.Code, sessResp.Msg)
	}
	var sess struct {
		ID string `json:"id"`
	}
	json.Unmarshal(sessResp.Data, &sess)
	return fac.ID, sess.ID
}

// createMemberForReg creates a member user and returns a logged-in client and the user ID.
func createMemberForReg(t *testing.T, admin *apiClient, prefix string) (*apiClient, string) {
	t.Helper()
	username := fmt.Sprintf("%s_%d_%d", prefix, os.Getpid(), time.Now().UnixMilli())
	resp := admin.post("/api/admin/users", map[string]interface{}{
		"username":     username,
		"password":     "TestPass123!",
		"role":         "member",
		"display_name": "Reg Test Member",
	})
	if resp.Code != 201 {
		t.Fatalf("Create member failed: %d %s", resp.Code, resp.Msg)
	}
	var u struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &u)

	mc := newClient(t)
	mc.login(username, "TestPass123!")
	return mc, u.ID
}

// registerAndApprove creates a registration for the given session, then has
// admin approve it. Returns the registration ID.
func registerAndApprove(t *testing.T, admin *apiClient, member *apiClient, sessionID string) string {
	t.Helper()

	resp := member.post("/api/registrations", map[string]interface{}{
		"session_id": sessionID,
	})
	if resp.Code != 201 {
		t.Fatalf("Create registration failed: %d %s", resp.Code, resp.Msg)
	}
	var reg struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	json.Unmarshal(resp.Data, &reg)
	if reg.Status != "pending" {
		t.Fatalf("Expected pending registration, got %q", reg.Status)
	}

	approveResp := admin.put(fmt.Sprintf("/api/admin/registrations/%s/approve", reg.ID), nil)
	if approveResp.Code != 200 {
		t.Fatalf("Approve registration failed: %d %s", approveResp.Code, approveResp.Msg)
	}
	return reg.ID
}

// TestCancelWaitlistedRegistration exercises the full lifecycle:
// register → approve → confirm (waitlisted because seat taken) → cancel.
// Uses a 1-seat session where the first user takes the seat, forcing the
// second user onto the waitlist.
func TestCancelWaitlistedRegistration(t *testing.T) {
	admin := getAdminClient(t)
	member1, _ := createMemberForReg(t, admin, "wl_seat")
	member2, _ := createMemberForReg(t, admin, "wl_cancel")

	// Session with exactly 1 seat
	_, sessionID := createOpenSession(t, admin, 1)

	// Member1 takes the only seat
	reg1ID := registerAndApprove(t, admin, member1, sessionID)
	confirm1 := member1.put(fmt.Sprintf("/api/registrations/%s/confirm", reg1ID), nil)
	if confirm1.Code != 200 {
		t.Fatalf("Member1 confirm failed: %d %s", confirm1.Code, confirm1.Msg)
	}
	var c1 struct {
		Status string `json:"status"`
	}
	json.Unmarshal(confirm1.Data, &c1)
	if c1.Status != "registered" {
		t.Fatalf("Member1 expected registered, got %q", c1.Status)
	}

	// Member2 tries to confirm → no seats → waitlisted
	reg2ID := registerAndApprove(t, admin, member2, sessionID)
	confirm2 := member2.put(fmt.Sprintf("/api/registrations/%s/confirm", reg2ID), nil)
	if confirm2.Code != 200 {
		t.Fatalf("Member2 confirm failed: %d %s", confirm2.Code, confirm2.Msg)
	}
	var c2 struct {
		Status string `json:"status"`
	}
	json.Unmarshal(confirm2.Data, &c2)
	if c2.Status != "waitlisted" {
		t.Fatalf("Member2 expected waitlisted, got %q", c2.Status)
	}

	// Cancel from waitlisted state
	cancelResp := member2.put(fmt.Sprintf("/api/registrations/%s/cancel", reg2ID), nil)
	if cancelResp.Code != 200 {
		t.Fatalf("Cancel waitlisted registration failed: %d %s", cancelResp.Code, cancelResp.Msg)
	}
	var canceled struct {
		Status string `json:"status"`
	}
	json.Unmarshal(cancelResp.Data, &canceled)
	if canceled.Status != "canceled" {
		t.Errorf("Expected canceled status, got %q", canceled.Status)
	}
}

// TestCancelPendingRegistration verifies cancellation from pending state.
func TestCancelPendingRegistration(t *testing.T) {
	admin := getAdminClient(t)
	member, _ := createMemberForReg(t, admin, "pend_cancel")

	_, sessionID := createOpenSession(t, admin, 10)

	resp := member.post("/api/registrations", map[string]interface{}{
		"session_id": sessionID,
	})
	if resp.Code != 201 {
		t.Fatalf("Create registration failed: %d %s", resp.Code, resp.Msg)
	}
	var reg struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &reg)

	cancelResp := member.put(fmt.Sprintf("/api/registrations/%s/cancel", reg.ID), nil)
	if cancelResp.Code != 200 {
		t.Fatalf("Cancel pending registration failed: %d %s", cancelResp.Code, cancelResp.Msg)
	}
	var canceled struct {
		Status string `json:"status"`
	}
	json.Unmarshal(cancelResp.Data, &canceled)
	if canceled.Status != "canceled" {
		t.Errorf("Expected canceled, got %q", canceled.Status)
	}
}

// TestCancelApprovedRegistration verifies cancellation from approved state.
func TestCancelApprovedRegistration(t *testing.T) {
	admin := getAdminClient(t)
	member, _ := createMemberForReg(t, admin, "appr_cancel")

	_, sessionID := createOpenSession(t, admin, 10)
	regID := registerAndApprove(t, admin, member, sessionID)

	cancelResp := member.put(fmt.Sprintf("/api/registrations/%s/cancel", regID), nil)
	if cancelResp.Code != 200 {
		t.Fatalf("Cancel approved registration failed: %d %s", cancelResp.Code, cancelResp.Msg)
	}
	var canceled struct {
		Status string `json:"status"`
	}
	json.Unmarshal(cancelResp.Data, &canceled)
	if canceled.Status != "canceled" {
		t.Errorf("Expected canceled, got %q", canceled.Status)
	}
}

// TestCancelRegisteredRegistration verifies cancellation from registered state
// and that the seat is released back.
func TestCancelRegisteredRegistration(t *testing.T) {
	admin := getAdminClient(t)
	member, _ := createMemberForReg(t, admin, "reg_cancel")

	_, sessionID := createOpenSession(t, admin, 10)
	regID := registerAndApprove(t, admin, member, sessionID)

	// Confirm → should be registered (seats available)
	confirmResp := member.put(fmt.Sprintf("/api/registrations/%s/confirm", regID), nil)
	if confirmResp.Code != 200 {
		t.Fatalf("Confirm registration failed: %d %s", confirmResp.Code, confirmResp.Msg)
	}
	var confirmed struct {
		Status string `json:"status"`
	}
	json.Unmarshal(confirmResp.Data, &confirmed)
	if confirmed.Status != "registered" {
		t.Fatalf("Expected registered status, got %q", confirmed.Status)
	}

	cancelResp := member.put(fmt.Sprintf("/api/registrations/%s/cancel", regID), nil)
	if cancelResp.Code != 200 {
		t.Fatalf("Cancel registered registration failed: %d %s", cancelResp.Code, cancelResp.Msg)
	}
	var canceled struct {
		Status string `json:"status"`
	}
	json.Unmarshal(cancelResp.Data, &canceled)
	if canceled.Status != "canceled" {
		t.Errorf("Expected canceled, got %q", canceled.Status)
	}
}

// TestCancelAlreadyCanceledRegistration verifies that double-cancel is rejected.
func TestCancelAlreadyCanceledRegistration(t *testing.T) {
	admin := getAdminClient(t)
	member, _ := createMemberForReg(t, admin, "dbl_cancel")

	_, sessionID := createOpenSession(t, admin, 10)

	resp := member.post("/api/registrations", map[string]interface{}{
		"session_id": sessionID,
	})
	if resp.Code != 201 {
		t.Fatalf("Create registration failed: %d %s", resp.Code, resp.Msg)
	}
	var reg struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &reg)

	// First cancel
	cancelResp := member.put(fmt.Sprintf("/api/registrations/%s/cancel", reg.ID), nil)
	if cancelResp.Code != 200 {
		t.Fatalf("First cancel failed: %d %s", cancelResp.Code, cancelResp.Msg)
	}

	// Second cancel should fail
	cancelResp2 := member.put(fmt.Sprintf("/api/registrations/%s/cancel", reg.ID), nil)
	if cancelResp2.Code == 200 {
		t.Error("Double cancel should be rejected")
	}
	if cancelResp2.Code != 422 {
		t.Errorf("Expected 422 for double cancel, got %d: %s", cancelResp2.Code, cancelResp2.Msg)
	}
}

// TestCancelRegistrationRequiresAuth verifies unauthenticated cancel is rejected.
func TestCancelRegistrationRequiresAuth(t *testing.T) {
	c := newClient(t)
	resp := c.put("/api/registrations/00000000-0000-0000-0000-000000000000/cancel", nil)
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}

// TestWaitlistedRegistrationShowsInList verifies that waitlisted registrations
// appear in the user's registration list with correct status.
func TestWaitlistedRegistrationShowsInList(t *testing.T) {
	admin := getAdminClient(t)
	seatTaker, _ := createMemberForReg(t, admin, "wl_list_s")
	member, _ := createMemberForReg(t, admin, "wl_list")

	// 1-seat session; first user takes the seat
	_, sessionID := createOpenSession(t, admin, 1)

	stRegID := registerAndApprove(t, admin, seatTaker, sessionID)
	seatTaker.put(fmt.Sprintf("/api/registrations/%s/confirm", stRegID), nil)

	// Second user → waitlisted
	regID := registerAndApprove(t, admin, member, sessionID)
	confirmResp := member.put(fmt.Sprintf("/api/registrations/%s/confirm", regID), nil)
	if confirmResp.Code != 200 {
		t.Fatalf("Confirm failed: %d %s", confirmResp.Code, confirmResp.Msg)
	}

	// List registrations filtered by waitlisted status
	listResp := member.get("/api/registrations?status=waitlisted")
	if listResp.Code != 200 {
		t.Fatalf("List registrations failed: %d %s", listResp.Code, listResp.Msg)
	}

	var data struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	json.Unmarshal(listResp.Data, &data)

	found := false
	for _, item := range data.Items {
		if item.ID == regID {
			found = true
			if item.Status != "waitlisted" {
				t.Errorf("Expected status waitlisted, got %q", item.Status)
			}
		}
	}
	if !found {
		t.Error("Waitlisted registration not found in filtered list")
	}
}
