//go:build integration

package api_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// getOrCreateAddress ensures the authenticated client has at least one address
// and returns its ID.
func getOrCreateAddress(t *testing.T, c *apiClient) string {
	t.Helper()
	resp := c.get("/api/addresses")
	if resp.Code != 200 {
		t.Fatalf("List addresses failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &data)

	if len(data.Items) > 0 {
		return data.Items[0].ID
	}

	// Create one
	resp = c.post("/api/addresses", map[string]interface{}{
		"label":          "Test Address",
		"recipient_name": "Buy Now Test",
		"phone":          "13800138000",
		"address_line1":  "123 Test St",
		"city":           "Test City",
		"province":       "Test Province",
		"postal_code":    "100000",
	})
	if resp.Code != 201 {
		t.Fatalf("Create address failed: %d %s", resp.Code, resp.Msg)
	}

	var addr struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &addr)
	if addr.ID == "" {
		t.Fatal("Created address has empty ID")
	}
	return addr.ID
}

// getActiveProduct returns an active product with sufficient stock.
func getActiveProduct(t *testing.T, c *apiClient, minStock int) *struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	PriceCents    int    `json:"price_cents"`
	StockQuantity int    `json:"stock_quantity"`
	Status        string `json:"status"`
	IsShippable   bool   `json:"is_shippable"`
} {
	t.Helper()
	resp := c.get("/api/products?page_size=10")
	if resp.Code != 200 {
		t.Fatalf("List products failed: %d %s", resp.Code, resp.Msg)
	}

	var prodData struct {
		Items []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			PriceCents    int    `json:"price_cents"`
			StockQuantity int    `json:"stock_quantity"`
			Status        string `json:"status"`
			IsShippable   bool   `json:"is_shippable"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &prodData)

	for i, p := range prodData.Items {
		if p.Status == "active" && p.StockQuantity >= minStock {
			return &prodData.Items[i]
		}
	}
	return nil
}

// TestBuyNowOrderCreation verifies that orders can be created with source=buy_now.
func TestBuyNowOrderCreation(t *testing.T) {
	c := getAdminClient(t)

	product := getActiveProduct(t, c, 1)
	if product == nil {
		t.Skip("No active in-stock product available")
	}

	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": product.ID, "quantity": 1},
		},
		"source": "buy_now",
	}
	if product.IsShippable {
		addrID := getOrCreateAddress(t, c)
		body["shipping_address_id"] = addrID
	}

	resp := c.post("/api/orders", body)
	if resp.Code != 201 {
		t.Fatalf("Buy Now order creation failed: %d %s", resp.Code, resp.Msg)
	}

	var orderData struct {
		ID          string `json:"id"`
		OrderNumber string `json:"order_number"`
		Status      string `json:"status"`
		TotalCents  int    `json:"total_cents"`
	}
	json.Unmarshal(resp.Data, &orderData)

	if orderData.ID == "" {
		t.Error("Order ID is empty")
	}
	if orderData.Status != "pending_payment" {
		t.Errorf("Order status = %q, want %q", orderData.Status, "pending_payment")
	}
	if orderData.TotalCents != product.PriceCents {
		t.Errorf("Order total = %d, want %d", orderData.TotalCents, product.PriceCents)
	}
}

// TestBuyNowInvalidSource verifies that invalid source values are rejected.
func TestBuyNowInvalidSource(t *testing.T) {
	c := getAdminClient(t)

	resp := c.post("/api/orders", map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": "fake-id", "quantity": 1},
		},
		"source": "direct",
	})
	if resp.Code == 201 {
		t.Fatal("Expected invalid source to be rejected")
	}
	if resp.Code != 400 {
		t.Errorf("Expected 400, got %d: %s", resp.Code, resp.Msg)
	}
}

// TestBuyNowEmptySource verifies that empty source is rejected.
func TestBuyNowEmptySource(t *testing.T) {
	c := getAdminClient(t)

	resp := c.post("/api/orders", map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": "fake-id", "quantity": 1},
		},
		"source": "",
	})
	if resp.Code == 201 {
		t.Fatal("Expected empty source to be rejected")
	}
}

// TestBuyNowDoesNotClearCart verifies that buy_now orders do not clear cart items.
func TestBuyNowDoesNotClearCart(t *testing.T) {
	c := getAdminClient(t)

	product := getActiveProduct(t, c, 2)
	if product == nil {
		t.Skip("Need active product with stock >= 2")
	}

	// Add item to cart
	resp := c.post("/api/cart", map[string]interface{}{
		"product_id": product.ID,
		"quantity":   1,
	})
	if resp.Code != 201 {
		t.Fatalf("Add to cart failed: %d %s", resp.Code, resp.Msg)
	}

	// Place buy_now order
	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": product.ID, "quantity": 1},
		},
		"source": "buy_now",
	}
	if product.IsShippable {
		addrID := getOrCreateAddress(t, c)
		body["shipping_address_id"] = addrID
	}

	resp = c.post("/api/orders", body)
	if resp.Code != 201 {
		t.Fatalf("Buy Now order failed: %d %s", resp.Code, resp.Msg)
	}

	// Check cart still has items
	resp = c.get("/api/cart")
	if resp.Code != 200 {
		t.Fatalf("Get cart failed: %d", resp.Code)
	}

	var cartData struct {
		Items []interface{} `json:"items"`
	}
	json.Unmarshal(resp.Data, &cartData)

	if len(cartData.Items) == 0 {
		t.Error("Cart should not be empty after buy_now order — buy_now must not clear cart")
	}
}

// TestBuyNowWithQuantity verifies buy now with quantity > 1.
func TestBuyNowWithQuantity(t *testing.T) {
	c := getAdminClient(t)

	product := getActiveProduct(t, c, 2)
	if product == nil {
		t.Skip("Need active product with stock >= 2")
	}

	qty := 2
	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": product.ID, "quantity": qty},
		},
		"source": "buy_now",
	}
	if product.IsShippable {
		addrID := getOrCreateAddress(t, c)
		body["shipping_address_id"] = addrID
	}

	resp := c.post("/api/orders", body)
	if resp.Code != 201 {
		t.Fatalf("Buy Now qty=2 failed: %d %s", resp.Code, resp.Msg)
	}

	var orderData struct {
		TotalCents int `json:"total_cents"`
	}
	json.Unmarshal(resp.Data, &orderData)

	expectedTotal := product.PriceCents * qty
	if orderData.TotalCents != expectedTotal {
		t.Errorf("Total = %d, want %d (price=%d x qty=%d)", orderData.TotalCents, expectedTotal, product.PriceCents, qty)
	}
}

// TestBuyNowRequiresAuth verifies buy now requires authentication.
func TestBuyNowRequiresAuth(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/orders", map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": "fake-id", "quantity": 1},
		},
		"source": "buy_now",
	})
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}

// TestBuyNowBannedUser verifies banned users cannot use buy now.
func TestBuyNowBannedUser(t *testing.T) {
	c := getAdminClient(t)

	// Create a user
	username := fmt.Sprintf("banned_buyer_%d", os.Getpid())
	resp := c.post("/api/admin/users", map[string]interface{}{
		"username":     username,
		"password":     "BannedTest123!",
		"role":         "member",
		"display_name": "Banned Buyer",
	})
	if resp.Code != 201 {
		t.Fatalf("Create user failed: %d %s", resp.Code, resp.Msg)
	}

	var userData struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &userData)

	// Login as the user first (while still active)
	bc := newClient(t)
	bc.login(username, "BannedTest123!")

	// Now ban the user
	resp = c.put(fmt.Sprintf("/api/admin/users/%s/status", userData.ID), map[string]interface{}{
		"status": "banned",
	})
	if resp.Code != 200 {
		t.Fatalf("Ban user failed: %d %s", resp.Code, resp.Msg)
	}

	// Try buy now with the token obtained before ban
	// The order service checks user status at creation time, so this should fail
	resp = bc.post("/api/orders", map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": "fake-id", "quantity": 1},
		},
		"source": "buy_now",
	})
	if resp.Code == 201 {
		t.Fatal("Banned user should not be able to create buy_now orders")
	}
	if resp.Code != 403 {
		t.Errorf("Expected 403 for banned user, got %d: %s", resp.Code, resp.Msg)
	}
}
