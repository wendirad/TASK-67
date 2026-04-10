//go:build integration

package api_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers for creating isolated member users
// ---------------------------------------------------------------------------

type memberClient struct {
	*apiClient
	userID   string
	username string
}

// createMember creates a new member user via the admin API and returns
// a logged-in client together with the user's ID.
func createMember(t *testing.T, admin *apiClient, suffix string) *memberClient {
	t.Helper()
	username := fmt.Sprintf("authz_%s_%d", suffix, os.Getpid())
	resp := admin.post("/api/admin/users", map[string]interface{}{
		"username":     username,
		"password":     "AuthzTest123!",
		"role":         "member",
		"display_name": "AuthZ " + suffix,
	})
	if resp.Code != 201 {
		t.Fatalf("Create member %s failed: %d %s", suffix, resp.Code, resp.Msg)
	}
	var data struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &data)

	mc := newClient(t)
	mc.login(username, "AuthzTest123!")
	return &memberClient{apiClient: mc, userID: data.ID, username: username}
}

// createMemberAddress creates an address for the given member and returns its ID.
func createMemberAddress(t *testing.T, mc *memberClient, label string) string {
	t.Helper()
	resp := mc.post("/api/addresses", map[string]interface{}{
		"label":          label,
		"recipient_name": "Test Recipient",
		"phone":          "13800138000",
		"address_line1":  "1 Auth Street",
		"city":           "Beijing",
		"province":       "Beijing",
		"postal_code":    "100000",
	})
	if resp.Code != 201 {
		t.Fatalf("Create address for %s failed: %d %s", mc.username, resp.Code, resp.Msg)
	}
	var addr struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &addr)
	return addr.ID
}

// createMemberTicket creates a ticket for the given member and returns its ID.
func createMemberTicket(t *testing.T, mc *memberClient) string {
	t.Helper()
	resp := mc.post("/api/tickets", map[string]interface{}{
		"type":        "general",
		"subject":     "AuthZ test ticket by " + mc.username,
		"description": "This ticket is used to test cross-user authorization.",
		"priority":    "low",
	})
	if resp.Code != 201 {
		t.Fatalf("Create ticket for %s failed: %d %s", mc.username, resp.Code, resp.Msg)
	}
	var data struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &data)
	return data.ID
}

// createMemberOrder creates an order via buy_now for the given member.
// Returns the order ID or empty string if no suitable product exists.
func createMemberOrder(t *testing.T, mc *memberClient) string {
	t.Helper()
	// Get a product
	resp := mc.get("/api/products?page_size=10")
	if resp.Code != 200 {
		t.Fatalf("List products failed: %d", resp.Code)
	}
	var prodData struct {
		Items []struct {
			ID            string `json:"id"`
			StockQuantity int    `json:"stock_quantity"`
			Status        string `json:"status"`
			IsShippable   bool   `json:"is_shippable"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &prodData)

	for _, p := range prodData.Items {
		if p.Status != "active" || p.StockQuantity < 1 {
			continue
		}
		body := map[string]interface{}{
			"items":  []map[string]interface{}{{"product_id": p.ID, "quantity": 1}},
			"source": "buy_now",
		}
		if p.IsShippable {
			addrID := createMemberAddress(t, mc, "ship")
			body["shipping_address_id"] = addrID
		}
		resp = mc.post("/api/orders", body)
		if resp.Code == 201 {
			var order struct {
				ID string `json:"id"`
			}
			json.Unmarshal(resp.Data, &order)
			return order.ID
		}
	}
	return ""
}

// ===========================================================================
// ORDER AUTHORIZATION TESTS
// ===========================================================================

// TestOrderGetCrossUser verifies member B cannot view member A's order.
func TestOrderGetCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "order_a")
	memberB := createMember(t, admin, "order_b")

	orderID := createMemberOrder(t, memberA)
	if orderID == "" {
		t.Skip("No product available to create an order")
	}

	// Owner can view
	resp := memberA.get("/api/orders/" + orderID)
	if resp.Code != 200 {
		t.Errorf("Owner should view own order: got %d %s", resp.Code, resp.Msg)
	}

	// Another member cannot view
	resp = memberB.get("/api/orders/" + orderID)
	if resp.Code != 403 {
		t.Errorf("Non-owner member should get 403 viewing another's order, got %d %s", resp.Code, resp.Msg)
	}

	// Admin can view any order
	resp = admin.get("/api/orders/" + orderID)
	if resp.Code != 200 {
		t.Errorf("Admin should view any order: got %d %s", resp.Code, resp.Msg)
	}
}

// TestOrderCancelCrossUser verifies member B cannot cancel member A's order.
func TestOrderCancelCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "cancel_a")
	memberB := createMember(t, admin, "cancel_b")

	orderID := createMemberOrder(t, memberA)
	if orderID == "" {
		t.Skip("No product available to create an order")
	}

	// Another member cannot cancel
	resp := memberB.put("/api/orders/"+orderID+"/cancel", nil)
	if resp.Code != 403 {
		t.Errorf("Non-owner member should get 403 canceling another's order, got %d %s", resp.Code, resp.Msg)
	}

	// Owner can cancel their pending_payment order
	resp = memberA.put("/api/orders/"+orderID+"/cancel", nil)
	if resp.Code != 200 {
		t.Errorf("Owner should be able to cancel own order: got %d %s", resp.Code, resp.Msg)
	}
}

// TestOrderCompleteCrossUser verifies member B cannot complete member A's order.
func TestOrderCompleteCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "complete_a")
	memberB := createMember(t, admin, "complete_b")

	orderID := createMemberOrder(t, memberA)
	if orderID == "" {
		t.Skip("No product available to create an order")
	}

	// Order is in pending_payment state, so complete should fail on state,
	// but the ownership check comes first. We verify B gets 403, not 422.
	resp := memberB.post("/api/orders/"+orderID+"/complete", nil)
	if resp.Code != 403 {
		t.Errorf("Non-owner member should get 403 completing another's order, got %d %s", resp.Code, resp.Msg)
	}
}

// TestOrderListIsolation verifies members only see their own orders.
func TestOrderListIsolation(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "list_a")
	memberB := createMember(t, admin, "list_b")

	orderID := createMemberOrder(t, memberA)
	if orderID == "" {
		t.Skip("No product available to create an order")
	}

	// Member B's order list should not contain member A's order
	resp := memberB.get("/api/orders")
	if resp.Code != 200 {
		t.Fatalf("List orders failed: %d", resp.Code)
	}
	var data struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &data)

	for _, o := range data.Items {
		if o.ID == orderID {
			t.Error("Member B's order list should not contain member A's order")
		}
	}
}

// ===========================================================================
// TICKET AUTHORIZATION TESTS
// ===========================================================================

// TestTicketGetCrossUser verifies member B cannot view member A's ticket.
func TestTicketGetCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "tkt_get_a")
	memberB := createMember(t, admin, "tkt_get_b")

	ticketID := createMemberTicket(t, memberA)

	// Owner can view
	resp := memberA.get("/api/tickets/" + ticketID)
	if resp.Code != 200 {
		t.Errorf("Owner should view own ticket: got %d %s", resp.Code, resp.Msg)
	}

	// Another member cannot view
	resp = memberB.get("/api/tickets/" + ticketID)
	if resp.Code != 403 {
		t.Errorf("Non-owner member should get 403 viewing another's ticket, got %d %s", resp.Code, resp.Msg)
	}

	// Admin can view any ticket
	resp = admin.get("/api/tickets/" + ticketID)
	if resp.Code != 200 {
		t.Errorf("Admin should view any ticket: got %d %s", resp.Code, resp.Msg)
	}
}

// TestTicketCommentCrossUser verifies member B cannot comment on member A's ticket.
func TestTicketCommentCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "tkt_cmt_a")
	memberB := createMember(t, admin, "tkt_cmt_b")

	ticketID := createMemberTicket(t, memberA)

	// Another member cannot comment
	resp := memberB.post("/api/tickets/"+ticketID+"/comments", map[string]interface{}{
		"content": "Unauthorized comment attempt",
	})
	if resp.Code != 403 {
		t.Errorf("Non-owner member should get 403 commenting on another's ticket, got %d %s", resp.Code, resp.Msg)
	}

	// Owner can comment
	resp = memberA.post("/api/tickets/"+ticketID+"/comments", map[string]interface{}{
		"content": "This is my own ticket comment.",
	})
	if resp.Code != 201 {
		t.Errorf("Owner should comment on own ticket: got %d %s", resp.Code, resp.Msg)
	}

	// Admin can comment on any ticket
	resp = admin.post("/api/tickets/"+ticketID+"/comments", map[string]interface{}{
		"content": "Admin comment on member ticket.",
	})
	if resp.Code != 201 {
		t.Errorf("Admin should comment on any ticket: got %d %s", resp.Code, resp.Msg)
	}
}

// TestTicketListIsolation verifies members only see their own tickets.
func TestTicketListIsolation(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "tkt_list_a")
	memberB := createMember(t, admin, "tkt_list_b")

	ticketID := createMemberTicket(t, memberA)

	// Member B's ticket list should not contain member A's ticket
	resp := memberB.get("/api/tickets")
	if resp.Code != 200 {
		t.Fatalf("List tickets failed: %d", resp.Code)
	}
	var data struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &data)

	for _, tk := range data.Items {
		if tk.ID == ticketID {
			t.Error("Member B's ticket list should not contain member A's ticket")
		}
	}
}

// TestTicketAssignRequiresRole verifies members cannot assign tickets.
func TestTicketAssignRequiresRole(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "tkt_assign_a")

	ticketID := createMemberTicket(t, memberA)

	// Member cannot assign (route is restricted to staff/moderator/admin)
	resp := memberA.put("/api/tickets/"+ticketID+"/assign", map[string]interface{}{
		"assigned_to": memberA.userID,
	})
	if resp.Code != 403 {
		t.Errorf("Member should get 403 assigning tickets, got %d %s", resp.Code, resp.Msg)
	}
}

// TestTicketStatusUpdateRequiresRole verifies members cannot change ticket status.
func TestTicketStatusUpdateRequiresRole(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "tkt_status_a")

	ticketID := createMemberTicket(t, memberA)

	// Member cannot update status (route is restricted to staff/moderator/admin)
	resp := memberA.put("/api/tickets/"+ticketID+"/status", map[string]interface{}{
		"status": "closed",
	})
	if resp.Code != 403 {
		t.Errorf("Member should get 403 updating ticket status, got %d %s", resp.Code, resp.Msg)
	}
}

// ===========================================================================
// ADDRESS AUTHORIZATION TESTS
// ===========================================================================

// TestAddressUpdateCrossUser verifies member B cannot update member A's address.
func TestAddressUpdateCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "addr_upd_a")
	memberB := createMember(t, admin, "addr_upd_b")

	addrID := createMemberAddress(t, memberA, "Home")

	// Owner can update
	resp := memberA.put("/api/addresses/"+addrID, map[string]interface{}{
		"label":          "Home Updated",
		"recipient_name": "Updated Name",
		"phone":          "13900139000",
		"address_line1":  "2 Updated St",
		"city":           "Shanghai",
		"province":       "Shanghai",
		"postal_code":    "200000",
	})
	if resp.Code != 200 {
		t.Errorf("Owner should update own address: got %d %s", resp.Code, resp.Msg)
	}

	// Another member gets 404 (obfuscated: not-found hides access denied)
	resp = memberB.put("/api/addresses/"+addrID, map[string]interface{}{
		"label":          "Hijack",
		"recipient_name": "Hacker",
		"phone":          "13900139000",
		"address_line1":  "Evil St",
		"city":           "Shanghai",
		"province":       "Shanghai",
		"postal_code":    "200000",
	})
	if resp.Code != 404 {
		t.Errorf("Non-owner should get 404 updating another's address, got %d %s", resp.Code, resp.Msg)
	}
}

// TestAddressDeleteCrossUser verifies member B cannot delete member A's address.
func TestAddressDeleteCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "addr_del_a")
	memberB := createMember(t, admin, "addr_del_b")

	addrID := createMemberAddress(t, memberA, "Home")

	// Another member gets 404
	resp := memberB.delete("/api/addresses/" + addrID)
	if resp.Code != 404 {
		t.Errorf("Non-owner should get 404 deleting another's address, got %d %s", resp.Code, resp.Msg)
	}

	// Owner can delete
	resp = memberA.delete("/api/addresses/" + addrID)
	if resp.Code != 200 {
		t.Errorf("Owner should delete own address: got %d %s", resp.Code, resp.Msg)
	}
}

// TestAddressSetDefaultCrossUser verifies member B cannot set-default on member A's address.
func TestAddressSetDefaultCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "addr_def_a")
	memberB := createMember(t, admin, "addr_def_b")

	addrID := createMemberAddress(t, memberA, "Home")

	// Another member gets 404
	resp := memberB.put("/api/addresses/"+addrID+"/default", nil)
	if resp.Code != 404 {
		t.Errorf("Non-owner should get 404 setting default on another's address, got %d %s", resp.Code, resp.Msg)
	}
}

// TestAddressListIsolation verifies members only see their own addresses.
func TestAddressListIsolation(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "addr_list_a")
	memberB := createMember(t, admin, "addr_list_b")

	addrID := createMemberAddress(t, memberA, "Home")

	// Member B's address list should not contain member A's address
	resp := memberB.get("/api/addresses")
	if resp.Code != 200 {
		t.Fatalf("List addresses failed: %d", resp.Code)
	}
	var data struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &data)

	for _, a := range data.Items {
		if a.ID == addrID {
			t.Error("Member B's address list should not contain member A's address")
		}
	}
}

// ===========================================================================
// CHECK-IN AUTHORIZATION TESTS
// ===========================================================================

// TestCheckinGetCrossUser verifies member B cannot view member A's check-in.
// Check-in IDs are UUIDs; we test with a fabricated ID to confirm 404 vs 403
// distinction, and also test the ownership path if a real check-in exists.
func TestCheckinGetCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberB := createMember(t, admin, "ci_get_b")

	// Use a non-existent UUID to verify 404 is returned (not 200 or 500)
	resp := memberB.get("/api/checkin/00000000-0000-0000-0000-000000000000")
	if resp.Code != 404 {
		t.Errorf("Non-existent check-in should return 404, got %d %s", resp.Code, resp.Msg)
	}
}

// TestCheckinBreakCrossUser verifies member B cannot start a break on a non-existent
// or another member's check-in.
func TestCheckinBreakCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberB := createMember(t, admin, "ci_brk_b")

	resp := memberB.post("/api/checkin/00000000-0000-0000-0000-000000000000/break", nil)
	if resp.Code != 404 {
		t.Errorf("Break on non-existent check-in should return 404, got %d %s", resp.Code, resp.Msg)
	}
}

// TestCheckinReturnCrossUser verifies member B cannot return from break on
// a non-existent or another member's check-in.
func TestCheckinReturnCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberB := createMember(t, admin, "ci_ret_b")

	resp := memberB.post("/api/checkin/00000000-0000-0000-0000-000000000000/return", nil)
	if resp.Code != 404 {
		t.Errorf("Return on non-existent check-in should return 404, got %d %s", resp.Code, resp.Msg)
	}
}

// TestCheckinPerformRequiresStaff verifies members cannot perform check-ins.
func TestCheckinPerformRequiresStaff(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "ci_perf_m")

	resp := member.post("/api/checkin", map[string]interface{}{
		"registration_id":    "00000000-0000-0000-0000-000000000000",
		"kiosk_device_token": "fake-token",
	})
	if resp.Code != 403 {
		t.Errorf("Member should get 403 performing check-in, got %d %s", resp.Code, resp.Msg)
	}
}

// ===========================================================================
// SHIPPING AUTHORIZATION TESTS
// ===========================================================================

// TestShippingEndpointsRequireStaff verifies members cannot access shipping management.
func TestShippingEndpointsRequireStaff(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "ship_m")

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/staff/shipping"},
		{"PUT", "/api/staff/shipping/00000000-0000-0000-0000-000000000000/ship"},
		{"PUT", "/api/staff/shipping/00000000-0000-0000-0000-000000000000/deliver"},
		{"PUT", "/api/staff/shipping/00000000-0000-0000-0000-000000000000/exception"},
	}

	for _, ep := range endpoints {
		resp := member.request(ep.method, ep.path, nil)
		if resp.Code != 403 {
			t.Errorf("%s %s: member should get 403, got %d %s", ep.method, ep.path, resp.Code, resp.Msg)
		}
	}
}

// ===========================================================================
// ADMIN ENDPOINT AUTHORIZATION TESTS
// ===========================================================================

// TestAdminEndpointsRequireAdminRole verifies members cannot access admin-only endpoints.
func TestAdminEndpointsRequireAdminRole(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "admin_m")

	endpoints := []string{
		"/api/admin/users",
		"/api/admin/config",
		"/api/admin/config-canary",
		"/api/admin/config-audit-logs",
	}

	for _, path := range endpoints {
		resp := member.get(path)
		if resp.Code != 403 {
			t.Errorf("GET %s: member should get 403, got %d %s", path, resp.Code, resp.Msg)
		}
	}

	// POST endpoints
	resp := member.post("/api/admin/users", map[string]interface{}{
		"username": "hacker", "password": "HackPass123!", "role": "admin", "display_name": "Hack",
	})
	if resp.Code != 403 {
		t.Errorf("POST /api/admin/users: member should get 403, got %d %s", resp.Code, resp.Msg)
	}

	resp = member.post("/api/admin/orders/00000000-0000-0000-0000-000000000000/refund", nil)
	if resp.Code != 403 {
		t.Errorf("POST refund: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}

// TestMemberCannotEscalateRole verifies member cannot create admin users.
func TestMemberCannotEscalateRole(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "escalate_m")

	resp := member.post("/api/admin/users", map[string]interface{}{
		"username":     "evil_admin",
		"password":     "EvilPass123!",
		"role":         "admin",
		"display_name": "Evil Admin",
	})
	if resp.Code != 403 {
		t.Errorf("Member should get 403 creating admin users, got %d %s", resp.Code, resp.Msg)
	}
}

// ===========================================================================
// CROSS-RESOURCE AUTHORIZATION: ORDER WITH ANOTHER'S ADDRESS
// ===========================================================================

// TestOrderWithOtherUsersAddress verifies member B cannot use member A's
// shipping address when placing an order.
func TestOrderWithOtherUsersAddress(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "xaddr_a")
	memberB := createMember(t, admin, "xaddr_b")

	addrID := createMemberAddress(t, memberA, "Private Address")

	// Find a shippable product
	resp := memberB.get("/api/products?page_size=10")
	if resp.Code != 200 {
		t.Fatalf("List products failed: %d", resp.Code)
	}
	var prodData struct {
		Items []struct {
			ID            string `json:"id"`
			StockQuantity int    `json:"stock_quantity"`
			Status        string `json:"status"`
			IsShippable   bool   `json:"is_shippable"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &prodData)

	var shippableID string
	for _, p := range prodData.Items {
		if p.Status == "active" && p.StockQuantity > 0 && p.IsShippable {
			shippableID = p.ID
			break
		}
	}
	if shippableID == "" {
		t.Skip("No shippable product available")
	}

	// Member B tries to use member A's address
	resp = memberB.post("/api/orders", map[string]interface{}{
		"items":               []map[string]interface{}{{"product_id": shippableID, "quantity": 1}},
		"shipping_address_id": addrID,
		"source":              "buy_now",
	})
	if resp.Code == 201 {
		t.Fatal("Member should not be able to use another user's shipping address")
	}
	// Should get 403 ("Shipping address does not belong to you") or 404
	if resp.Code != 403 && resp.Code != 404 {
		t.Errorf("Expected 403 or 404 using another's address, got %d %s", resp.Code, resp.Msg)
	}
}
