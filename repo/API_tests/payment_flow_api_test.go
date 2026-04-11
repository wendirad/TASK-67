//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

// TestPaymentSuccessTransition creates an order, simulates payment, and
// verifies the order transitions to paid.
func TestPaymentSuccessTransition(t *testing.T) {
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
		body["shipping_address_id"] = getOrCreateAddress(t, c)
	}

	resp := c.post("/api/orders", body)
	if resp.Code != 201 {
		t.Fatalf("Create order failed: %d %s", resp.Code, resp.Msg)
	}

	var order struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	json.Unmarshal(resp.Data, &order)

	if order.Status != "pending_payment" {
		t.Fatalf("New order status = %q, want pending_payment", order.Status)
	}

	// Simulate payment
	simResp := c.post("/api/payments/"+order.ID+"/simulate-callback", nil)
	if simResp.Code != 200 {
		t.Fatalf("Simulate payment failed: %d %s", simResp.Code, simResp.Msg)
	}

	// Verify order is now paid
	getResp := c.get("/api/orders/" + order.ID)
	if getResp.Code != 200 {
		t.Fatalf("Get order failed: %d %s", getResp.Code, getResp.Msg)
	}

	var updated struct {
		Status string `json:"status"`
	}
	json.Unmarshal(getResp.Data, &updated)
	if updated.Status != "paid" {
		t.Errorf("After payment, order status = %q, want paid", updated.Status)
	}
}

// TestPaymentDuplicateCallback verifies that calling simulate-callback twice
// on the same order is idempotent and does not error.
func TestPaymentDuplicateCallback(t *testing.T) {
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
		body["shipping_address_id"] = getOrCreateAddress(t, c)
	}

	resp := c.post("/api/orders", body)
	if resp.Code != 201 {
		t.Fatalf("Create order failed: %d %s", resp.Code, resp.Msg)
	}

	var order struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &order)

	// First payment
	first := c.post("/api/payments/"+order.ID+"/simulate-callback", nil)
	if first.Code != 200 {
		t.Fatalf("First simulate-callback failed: %d %s", first.Code, first.Msg)
	}

	// Second payment (duplicate) — should not fail
	second := c.post("/api/payments/"+order.ID+"/simulate-callback", nil)
	if second.Code != 200 {
		t.Errorf("Duplicate simulate-callback returned %d %s, want 200", second.Code, second.Msg)
	}
}

// TestPaymentCallbackWithRealOrder creates an order and sends a real callback
// with proper signature to test the full callback validation chain.
func TestPaymentCallbackWithRealOrder(t *testing.T) {
	merchantKey := readMerchantKey()
	if merchantKey == "" {
		t.Skip("Merchant key not available")
	}

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
		body["shipping_address_id"] = getOrCreateAddress(t, c)
	}

	resp := c.post("/api/orders", body)
	if resp.Code != 201 {
		t.Fatalf("Create order failed: %d %s", resp.Code, resp.Msg)
	}

	var order struct {
		ID          string `json:"id"`
		OrderNumber string `json:"order_number"`
		TotalCents  int    `json:"total_cents"`
	}
	json.Unmarshal(resp.Data, &order)

	// Construct a valid callback
	txnID := "WX-TXN-REAL-ORDER"
	nonceStr := "test-nonce-real"
	sign := computeSignature(txnID, order.OrderNumber, order.TotalCents, "SUCCESS", nonceStr, merchantKey)

	cbResp := c.post("/api/payments/callback", map[string]interface{}{
		"transaction_id": txnID,
		"order_number":   order.OrderNumber,
		"amount_cents":   order.TotalCents,
		"status":         "SUCCESS",
		"nonce_str":      nonceStr,
		"sign":           sign,
	})
	if cbResp.Code != 200 {
		t.Fatalf("Callback with real order failed: %d %s", cbResp.Code, cbResp.Msg)
	}

	// Verify order is now paid
	getResp := c.get("/api/orders/" + order.ID)
	if getResp.Code != 200 {
		t.Fatalf("Get order failed: %d", getResp.Code)
	}

	var updated struct {
		Status string `json:"status"`
	}
	json.Unmarshal(getResp.Data, &updated)
	if updated.Status != "paid" {
		t.Errorf("After real callback, order status = %q, want paid", updated.Status)
	}
}

// TestPaymentCanceledOrderRejectsCallback verifies that paying for a canceled
// order is rejected.
func TestPaymentCanceledOrderRejectsCallback(t *testing.T) {
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
		body["shipping_address_id"] = getOrCreateAddress(t, c)
	}

	resp := c.post("/api/orders", body)
	if resp.Code != 201 {
		t.Fatalf("Create order failed: %d %s", resp.Code, resp.Msg)
	}

	var order struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp.Data, &order)

	// Cancel the order first
	cancelResp := c.put("/api/orders/"+order.ID+"/cancel", nil)
	if cancelResp.Code != 200 {
		t.Fatalf("Cancel order failed: %d %s", cancelResp.Code, cancelResp.Msg)
	}

	// Now try to simulate payment on the canceled order
	simResp := c.post("/api/payments/"+order.ID+"/simulate-callback", nil)
	if simResp.Code == 200 {
		t.Fatal("Payment on canceled order should be rejected")
	}
}
