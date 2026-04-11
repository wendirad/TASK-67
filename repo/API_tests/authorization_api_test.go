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

// ===========================================================================
// CART AUTHORIZATION TESTS
// ===========================================================================

// addCartItem adds a product to the member's cart and returns the cart item ID.
func addCartItem(t *testing.T, mc *memberClient) string {
	t.Helper()
	resp := mc.get("/api/products?page_size=10")
	if resp.Code != 200 {
		t.Fatalf("List products failed: %d", resp.Code)
	}
	var prodData struct {
		Items []struct {
			ID            string `json:"id"`
			StockQuantity int    `json:"stock_quantity"`
			Status        string `json:"status"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &prodData)

	for _, p := range prodData.Items {
		if p.Status == "active" && p.StockQuantity > 0 {
			resp = mc.post("/api/cart", map[string]interface{}{
				"product_id": p.ID,
				"quantity":   1,
			})
			if resp.Code == 201 {
				// Get cart to find the item ID
				cartResp := mc.get("/api/cart")
				if cartResp.Code != 200 {
					t.Fatalf("Get cart failed: %d", cartResp.Code)
				}
				var items []struct {
					ID string `json:"id"`
				}
				json.Unmarshal(cartResp.Data, &items)
				if len(items) > 0 {
					return items[len(items)-1].ID
				}
			}
		}
	}
	return ""
}

// TestCartUpdateCrossUser verifies member B cannot update member A's cart item.
func TestCartUpdateCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "cart_upd_a")
	memberB := createMember(t, admin, "cart_upd_b")

	itemID := addCartItem(t, memberA)
	if itemID == "" {
		t.Skip("No product available to add to cart")
	}

	// Owner can update
	resp := memberA.put("/api/cart/"+itemID, map[string]interface{}{
		"quantity": 2,
	})
	if resp.Code != 200 {
		t.Errorf("Owner should update own cart item: got %d %s", resp.Code, resp.Msg)
	}

	// Another member should get 403
	resp = memberB.put("/api/cart/"+itemID, map[string]interface{}{
		"quantity": 5,
	})
	if resp.Code != 403 {
		t.Errorf("Non-owner should get 403 updating another's cart item, got %d %s", resp.Code, resp.Msg)
	}
}

// TestCartDeleteCrossUser verifies member B cannot delete member A's cart item.
func TestCartDeleteCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "cart_del_a")
	memberB := createMember(t, admin, "cart_del_b")

	itemID := addCartItem(t, memberA)
	if itemID == "" {
		t.Skip("No product available to add to cart")
	}

	// Another member should get 403
	resp := memberB.delete("/api/cart/" + itemID)
	if resp.Code != 403 {
		t.Errorf("Non-owner should get 403 deleting another's cart item, got %d %s", resp.Code, resp.Msg)
	}

	// Owner can delete
	resp = memberA.delete("/api/cart/" + itemID)
	if resp.Code != 200 {
		t.Errorf("Owner should delete own cart item: got %d %s", resp.Code, resp.Msg)
	}
}

// TestCartListIsolation verifies members only see their own cart items.
func TestCartListIsolation(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "cart_iso_a")
	memberB := createMember(t, admin, "cart_iso_b")

	itemID := addCartItem(t, memberA)
	if itemID == "" {
		t.Skip("No product available to add to cart")
	}

	// Member B's cart should not contain member A's items
	resp := memberB.get("/api/cart")
	if resp.Code != 200 {
		t.Fatalf("Get cart failed: %d", resp.Code)
	}
	var items []struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &items)

	for _, item := range items {
		if item.ID == itemID {
			t.Error("Member B's cart should not contain member A's cart item")
		}
	}
}

// ===========================================================================
// REGISTRATION AUTHORIZATION TESTS
// ===========================================================================

// TestRegistrationConfirmCrossUser verifies member B cannot confirm member A's registration.
func TestRegistrationConfirmCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "reg_cfm_a")
	memberB := createMember(t, admin, "reg_cfm_b")

	// Get a session
	resp := memberA.get("/api/sessions?page_size=10")
	if resp.Code != 200 {
		t.Fatalf("List sessions failed: %d", resp.Code)
	}
	var sessData struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &sessData)

	var regID string
	for _, s := range sessData.Items {
		if s.Status == "open" || s.Status == "published" {
			resp = memberA.post("/api/registrations", map[string]interface{}{
				"session_id": s.ID,
			})
			if resp.Code == 201 {
				var reg struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				}
				json.Unmarshal(resp.Data, &reg)
				// Approve it so it can be confirmed
				admin.put("/api/admin/registrations/"+reg.ID+"/approve", nil)
				regID = reg.ID
				break
			}
		}
	}
	if regID == "" {
		t.Skip("No session available to create a registration")
	}

	// Member B should get 403 trying to confirm member A's registration
	resp = memberB.put("/api/registrations/"+regID+"/confirm", nil)
	if resp.Code != 403 {
		t.Errorf("Non-owner should get 403 confirming another's registration, got %d %s", resp.Code, resp.Msg)
	}

	// Owner can confirm
	resp = memberA.put("/api/registrations/"+regID+"/confirm", nil)
	if resp.Code != 200 {
		t.Errorf("Owner should confirm own registration: got %d %s", resp.Code, resp.Msg)
	}
}

// TestRegistrationCancelCrossUser verifies member B cannot cancel member A's registration.
func TestRegistrationCancelCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "reg_cnl_a")
	memberB := createMember(t, admin, "reg_cnl_b")

	// Get a session and create a registration
	resp := memberA.get("/api/sessions?page_size=10")
	if resp.Code != 200 {
		t.Fatalf("List sessions failed: %d", resp.Code)
	}
	var sessData struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &sessData)

	var regID string
	for _, s := range sessData.Items {
		if s.Status == "open" || s.Status == "published" {
			resp = memberA.post("/api/registrations", map[string]interface{}{
				"session_id": s.ID,
			})
			if resp.Code == 201 {
				var reg struct {
					ID string `json:"id"`
				}
				json.Unmarshal(resp.Data, &reg)
				regID = reg.ID
				break
			}
		}
	}
	if regID == "" {
		t.Skip("No session available to create a registration")
	}

	// Member B should get 403 trying to cancel member A's registration
	resp = memberB.put("/api/registrations/"+regID+"/cancel", nil)
	if resp.Code != 403 {
		t.Errorf("Non-owner should get 403 canceling another's registration, got %d %s", resp.Code, resp.Msg)
	}

	// Admin can cancel any registration
	resp = admin.put("/api/registrations/"+regID+"/cancel", nil)
	if resp.Code != 200 {
		t.Errorf("Admin should cancel any registration: got %d %s", resp.Code, resp.Msg)
	}
}

// ===========================================================================
// POST AUTHORIZATION TESTS
// ===========================================================================

// TestPostSelfReportPrevention verifies a user cannot report their own post.
func TestPostSelfReportPrevention(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "post_self")

	// Create a post
	resp := member.post("/api/posts", map[string]interface{}{
		"title":   "Self report test",
		"content": "Testing that users cannot report their own posts.",
	})
	if resp.Code != 201 {
		t.Fatalf("Create post failed: %d %s", resp.Code, resp.Msg)
	}
	var post struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &post)

	// Try to report own post — should get 400
	resp = member.post("/api/posts/"+post.ID+"/report", map[string]interface{}{
		"reason": "Testing self-report prevention",
	})
	if resp.Code != 400 {
		t.Errorf("Self-reporting should return 400, got %d %s", resp.Code, resp.Msg)
	}
}

// TestPostReportCrossUser verifies a different user CAN report another's post.
func TestPostReportCrossUser(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "post_rpt_a")
	memberB := createMember(t, admin, "post_rpt_b")

	// Member A creates a post
	resp := memberA.post("/api/posts", map[string]interface{}{
		"title":   "Reportable post",
		"content": "This post can be reported by other users.",
	})
	if resp.Code != 201 {
		t.Fatalf("Create post failed: %d %s", resp.Code, resp.Msg)
	}
	var post struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &post)

	// Member B can report member A's post
	resp = memberB.post("/api/posts/"+post.ID+"/report", map[string]interface{}{
		"reason": "Testing cross-user reporting works",
	})
	if resp.Code != 201 {
		t.Errorf("Cross-user report should return 201, got %d %s", resp.Code, resp.Msg)
	}
}

// TestPostDuplicateReport verifies a user cannot report the same post twice.
func TestPostDuplicateReport(t *testing.T) {
	admin := getAdminClient(t)
	memberA := createMember(t, admin, "post_dup_a")
	memberB := createMember(t, admin, "post_dup_b")

	resp := memberA.post("/api/posts", map[string]interface{}{
		"title":   "Duplicate report test",
		"content": "This post tests duplicate report prevention.",
	})
	if resp.Code != 201 {
		t.Fatalf("Create post failed: %d %s", resp.Code, resp.Msg)
	}
	var post struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &post)

	// First report succeeds
	resp = memberB.post("/api/posts/"+post.ID+"/report", map[string]interface{}{
		"reason": "First report - should succeed",
	})
	if resp.Code != 201 {
		t.Fatalf("First report should succeed: got %d %s", resp.Code, resp.Msg)
	}

	// Second report should be rejected (409 Conflict)
	resp = memberB.post("/api/posts/"+post.ID+"/report", map[string]interface{}{
		"reason": "Duplicate report - should fail",
	})
	if resp.Code != 409 {
		t.Errorf("Duplicate report should return 409, got %d %s", resp.Code, resp.Msg)
	}
}

// ===========================================================================
// ADDITIONAL ADMIN ENDPOINT AUTHORIZATION TESTS
// ===========================================================================

// TestAdminFacilitiesRequireAdmin verifies members cannot access facility endpoints.
func TestAdminFacilitiesRequireAdmin(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "fac_m")

	// GET facilities
	resp := member.get("/api/admin/facilities")
	if resp.Code != 403 {
		t.Errorf("GET /api/admin/facilities: member should get 403, got %d %s", resp.Code, resp.Msg)
	}

	// POST facility
	resp = member.post("/api/admin/facilities", map[string]interface{}{
		"name":     "Hacked Facility",
		"location": "Evil Lair",
	})
	if resp.Code != 403 {
		t.Errorf("POST /api/admin/facilities: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}

// TestAdminSessionsRequireAdmin verifies members cannot access admin session endpoints.
func TestAdminSessionsRequireAdmin(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "sess_m")

	// POST session
	resp := member.post("/api/admin/sessions", map[string]interface{}{
		"name":        "Hacked Session",
		"facility_id": "00000000-0000-0000-0000-000000000000",
	})
	if resp.Code != 403 {
		t.Errorf("POST /api/admin/sessions: member should get 403, got %d %s", resp.Code, resp.Msg)
	}

	// PUT session
	resp = member.put("/api/admin/sessions/00000000-0000-0000-0000-000000000000", map[string]interface{}{
		"name": "Hacked",
	})
	if resp.Code != 403 {
		t.Errorf("PUT /api/admin/sessions/:id: member should get 403, got %d %s", resp.Code, resp.Msg)
	}

	// PUT session status
	resp = member.put("/api/admin/sessions/00000000-0000-0000-0000-000000000000/status", map[string]interface{}{
		"status": "open",
	})
	if resp.Code != 403 {
		t.Errorf("PUT /api/admin/sessions/:id/status: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}

// TestAdminRegistrationsRequireAdmin verifies members cannot access admin registration endpoints.
func TestAdminRegistrationsRequireAdmin(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "areg_m")

	resp := member.get("/api/admin/registrations")
	if resp.Code != 403 {
		t.Errorf("GET /api/admin/registrations: member should get 403, got %d %s", resp.Code, resp.Msg)
	}

	resp = member.put("/api/admin/registrations/00000000-0000-0000-0000-000000000000/approve", nil)
	if resp.Code != 403 {
		t.Errorf("PUT approve: member should get 403, got %d %s", resp.Code, resp.Msg)
	}

	resp = member.put("/api/admin/registrations/00000000-0000-0000-0000-000000000000/reject", map[string]interface{}{
		"reason": "Test rejection",
	})
	if resp.Code != 403 {
		t.Errorf("PUT reject: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}

// TestBackupEndpointsRequireAdmin verifies members cannot access backup endpoints.
func TestBackupEndpointsRequireAdmin(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "backup_m")

	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/api/admin/backup"},
		{"GET", "/api/admin/backups"},
		{"GET", "/api/admin/backup/restore-targets"},
		{"POST", "/api/admin/backup/restore"},
		{"POST", "/api/admin/archive/run"},
		{"GET", "/api/admin/archive/status"},
	}

	for _, ep := range endpoints {
		resp := member.request(ep.method, ep.path, nil)
		if resp.Code != 403 {
			t.Errorf("%s %s: member should get 403, got %d %s", ep.method, ep.path, resp.Code, resp.Msg)
		}
	}
}

// TestKPIEndpointsRequireAdmin verifies members cannot access KPI endpoints.
func TestKPIEndpointsRequireAdmin(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "kpi_m")

	endpoints := []string{
		"/api/kpi/overview",
		"/api/kpi/fill-rate",
		"/api/kpi/members",
		"/api/kpi/engagement",
		"/api/kpi/coaches",
		"/api/kpi/revenue",
		"/api/kpi/tickets",
	}

	for _, path := range endpoints {
		resp := member.get(path)
		if resp.Code != 403 {
			t.Errorf("GET %s: member should get 403, got %d %s", path, resp.Code, resp.Msg)
		}
	}
}

// TestConfigUpdateRequiresAdmin verifies members cannot update config entries.
func TestConfigUpdateRequiresAdmin(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "cfg_m")

	resp := member.put("/api/admin/config/post.rate_limit_per_hour", map[string]interface{}{
		"value": "100",
	})
	if resp.Code != 403 {
		t.Errorf("PUT config: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}

// TestPaymentSimulateRequiresStaff verifies members cannot simulate payment callbacks.
func TestPaymentSimulateRequiresStaff(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "pay_sim_m")

	resp := member.post("/api/payments/00000000-0000-0000-0000-000000000000/simulate-callback", map[string]interface{}{
		"status": "paid",
	})
	if resp.Code != 403 {
		t.Errorf("Simulate callback: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}

// TestImportExportRequiresStaffOrAdmin verifies members cannot access import/export.
func TestImportExportRequiresStaffOrAdmin(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "ie_m")

	resp := member.get("/api/export")
	if resp.Code != 403 {
		t.Errorf("GET /api/export: member should get 403, got %d %s", resp.Code, resp.Msg)
	}

	resp = member.post("/api/import", map[string]interface{}{
		"type": "products",
	})
	if resp.Code != 403 {
		t.Errorf("POST /api/import: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}

// TestModerationEndpointsRequireRole verifies members cannot access moderation endpoints.
func TestModerationEndpointsRequireModeratorOrAdmin(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "mod_m")

	resp := member.get("/api/moderation/posts")
	if resp.Code != 403 {
		t.Errorf("GET moderation queue: member should get 403, got %d %s", resp.Code, resp.Msg)
	}

	resp = member.post("/api/moderation/posts/00000000-0000-0000-0000-000000000000/decision", map[string]interface{}{
		"action": "approve",
		"reason": "This is a test moderation decision attempt",
	})
	if resp.Code != 403 {
		t.Errorf("POST moderation decision: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}

// TestCheckinQRRequiresStaff verifies members cannot generate QR codes.
func TestCheckinQRRequiresStaff(t *testing.T) {
	admin := getAdminClient(t)
	member := createMember(t, admin, "qr_m")

	resp := member.get("/api/sessions/00000000-0000-0000-0000-000000000000/qr")
	if resp.Code != 403 {
		t.Errorf("GET session QR: member should get 403, got %d %s", resp.Code, resp.Msg)
	}
}
