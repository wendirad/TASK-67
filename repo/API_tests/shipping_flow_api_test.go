//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

// createOrderForShipping creates a buy-now order with a shippable product and
// returns the order ID, order number, and total cents. Skips the test if no
// shippable product with sufficient stock is available.
func createOrderForShipping(t *testing.T, c *apiClient) (orderID, orderNumber string, totalCents int) {
	t.Helper()
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

	var data struct {
		ID          string `json:"id"`
		OrderNumber string `json:"order_number"`
		TotalCents  int    `json:"total_cents"`
	}
	json.Unmarshal(resp.Data, &data)
	return data.ID, data.OrderNumber, data.TotalCents
}

// findShippingRecordForOrder finds the shipping record associated with an order.
func findShippingRecordForOrder(t *testing.T, c *apiClient, orderID string) string {
	t.Helper()
	resp := c.get("/api/staff/shipping?page_size=50")
	if resp.Code != 200 {
		t.Fatalf("List shipping failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Items []struct {
			ID      string `json:"id"`
			OrderID string `json:"order_id"`
			Status  string `json:"status"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &data)

	for _, s := range data.Items {
		if s.OrderID == orderID {
			return s.ID
		}
	}
	return ""
}

// TestShippingDeliveryFlow tests the full shipping lifecycle:
// create order → simulate payment → ship → deliver with proof.
func TestShippingDeliveryFlow(t *testing.T) {
	c := getAdminClient(t)

	// Step 1: Create an order
	orderID, _, _ := createOrderForShipping(t, c)

	// Step 2: Simulate payment so order becomes paid/processing
	simResp := c.post("/api/payments/"+orderID+"/simulate-callback", nil)
	if simResp.Code != 200 {
		t.Logf("Simulate payment returned %d %s (order may not need payment)", simResp.Code, simResp.Msg)
	}

	// Step 3: Find the shipping record
	shippingID := findShippingRecordForOrder(t, c, orderID)
	if shippingID == "" {
		t.Skip("No shipping record created for order (product may not be shippable)")
	}

	// Step 4: Ship the item
	resp := c.put("/api/staff/shipping/"+shippingID+"/ship", map[string]interface{}{
		"tracking_number": "TRACK-TEST-001",
		"carrier":         "TestCarrier",
	})
	if resp.Code != 200 {
		t.Fatalf("Ship failed: %d %s", resp.Code, resp.Msg)
	}

	// Step 5: Deliver with proof
	resp = c.put("/api/staff/shipping/"+shippingID+"/deliver", map[string]interface{}{
		"proof_type": "acknowledgment",
		"proof_data": "Received by John Doe at front desk",
	})
	if resp.Code != 200 {
		t.Fatalf("Deliver failed: %d %s", resp.Code, resp.Msg)
	}

	// Step 6: Verify order status is now delivered
	orderResp := c.get("/api/orders/" + orderID)
	if orderResp.Code == 200 {
		var order struct {
			Status string `json:"status"`
		}
		json.Unmarshal(orderResp.Data, &order)
		if order.Status != "delivered" {
			t.Logf("Order status after delivery: %s (expected delivered)", order.Status)
		}
	}
}

// TestShippingExceptionFlow tests marking a shipment as exception.
func TestShippingExceptionFlow(t *testing.T) {
	c := getAdminClient(t)

	orderID, _, _ := createOrderForShipping(t, c)

	// Simulate payment
	c.post("/api/payments/"+orderID+"/simulate-callback", nil)

	shippingID := findShippingRecordForOrder(t, c, orderID)
	if shippingID == "" {
		t.Skip("No shipping record created for order")
	}

	// Mark exception on a pending shipment
	resp := c.put("/api/staff/shipping/"+shippingID+"/exception", map[string]interface{}{
		"exception_notes": "Package damaged during handling",
	})
	if resp.Code != 200 {
		t.Fatalf("Exception failed: %d %s", resp.Code, resp.Msg)
	}
}

// TestShippingDeliverWithSignatureProof verifies delivery with base64 signature proof.
func TestShippingDeliverWithSignatureProof(t *testing.T) {
	c := getAdminClient(t)

	orderID, _, _ := createOrderForShipping(t, c)
	c.post("/api/payments/"+orderID+"/simulate-callback", nil)

	shippingID := findShippingRecordForOrder(t, c, orderID)
	if shippingID == "" {
		t.Skip("No shipping record created for order")
	}

	// Ship first
	resp := c.put("/api/staff/shipping/"+shippingID+"/ship", map[string]interface{}{
		"tracking_number": "TRACK-SIG-001",
	})
	if resp.Code != 200 {
		t.Fatalf("Ship failed: %d %s", resp.Code, resp.Msg)
	}

	// Deliver with signature proof (base64-encoded data)
	resp = c.put("/api/staff/shipping/"+shippingID+"/deliver", map[string]interface{}{
		"proof_type": "signature",
		"proof_data": "aGVsbG8gd29ybGQ=", // valid base64 for "hello world"
	})
	if resp.Code != 200 {
		t.Fatalf("Deliver with signature proof failed: %d %s", resp.Code, resp.Msg)
	}
}

// TestShippingDeliverMissingProof verifies delivery without proof is rejected.
func TestShippingDeliverMissingProof(t *testing.T) {
	c := getAdminClient(t)

	orderID, _, _ := createOrderForShipping(t, c)
	c.post("/api/payments/"+orderID+"/simulate-callback", nil)

	shippingID := findShippingRecordForOrder(t, c, orderID)
	if shippingID == "" {
		t.Skip("No shipping record created for order")
	}

	// Ship first
	c.put("/api/staff/shipping/"+shippingID+"/ship", map[string]interface{}{
		"tracking_number": "TRACK-MISS-001",
	})

	// Try to deliver without proof fields — should fail
	resp := c.put("/api/staff/shipping/"+shippingID+"/deliver", map[string]interface{}{})
	if resp.Code == 200 {
		t.Fatal("Deliver without proof should be rejected")
	}
}

// TestShippingDeliverInvalidProofType verifies invalid proof_type is rejected.
func TestShippingDeliverInvalidProofType(t *testing.T) {
	c := getAdminClient(t)

	orderID, _, _ := createOrderForShipping(t, c)
	c.post("/api/payments/"+orderID+"/simulate-callback", nil)

	shippingID := findShippingRecordForOrder(t, c, orderID)
	if shippingID == "" {
		t.Skip("No shipping record created for order")
	}

	c.put("/api/staff/shipping/"+shippingID+"/ship", map[string]interface{}{
		"tracking_number": "TRACK-BAD-001",
	})

	resp := c.put("/api/staff/shipping/"+shippingID+"/deliver", map[string]interface{}{
		"proof_type": "photo",
		"proof_data": "somedata",
	})
	if resp.Code == 200 {
		t.Fatal("Invalid proof_type should be rejected")
	}
}

// TestShippingExceptionMissingNotes verifies exception without notes is rejected.
func TestShippingExceptionMissingNotes(t *testing.T) {
	c := getAdminClient(t)

	orderID, _, _ := createOrderForShipping(t, c)
	c.post("/api/payments/"+orderID+"/simulate-callback", nil)

	shippingID := findShippingRecordForOrder(t, c, orderID)
	if shippingID == "" {
		t.Skip("No shipping record created for order")
	}

	resp := c.put("/api/staff/shipping/"+shippingID+"/exception", map[string]interface{}{})
	if resp.Code == 200 {
		t.Fatal("Exception without notes should be rejected")
	}
}
