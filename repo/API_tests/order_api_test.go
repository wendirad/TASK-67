//go:build integration

package api_tests

import (
	"testing"
)

func TestListOrders(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/orders")
	if resp.Code != 200 {
		t.Fatalf("List orders failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestCartEndpoints(t *testing.T) {
	c := getAdminClient(t)

	// Get cart (should be empty or have items)
	resp := c.get("/api/cart")
	if resp.Code != 200 {
		t.Fatalf("Get cart failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestCreateOrderWithoutItems(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/orders", map[string]interface{}{
		"items":  []interface{}{},
		"source": "buy_now",
	})
	if resp.Code == 201 {
		t.Fatal("Expected order creation to fail without items")
	}
}

func TestOrdersRequireAuth(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/orders")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
