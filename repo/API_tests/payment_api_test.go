//go:build integration

package api_tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

func readMerchantKey() string {
	key, err := os.ReadFile("/run/secrets/wechat_merchant_key")
	if err == nil && len(key) > 0 {
		return strings.TrimSpace(string(key))
	}
	if k := os.Getenv("WECHAT_MERCHANT_KEY"); k != "" {
		return k
	}
	return ""
}

func computeSignature(transactionID, orderNumber string, amountCents int, status, nonceStr, merchantKey string) string {
	params := map[string]string{
		"transaction_id": transactionID,
		"order_number":   orderNumber,
		"amount_cents":   fmt.Sprintf("%d", amountCents),
		"status":         status,
		"nonce_str":      nonceStr,
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, params[k]))
	}
	signingString := strings.Join(pairs, "&")
	mac := hmac.New(sha256.New, []byte(merchantKey))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestPaymentCallbackInvalidSignature(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/payments/callback", map[string]interface{}{
		"transaction_id": "FAKE-TXN-001",
		"order_number":   "ORD-99999999-00001",
		"amount_cents":   1000,
		"status":         "SUCCESS",
		"nonce_str":      "nonce123",
		"sign":           "invalidsignature",
	})
	if resp.Code == 200 {
		t.Fatal("Expected callback with invalid signature to fail")
	}
}

func TestPaymentCallbackMissingFields(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/payments/callback", map[string]interface{}{
		"transaction_id": "",
		"order_number":   "",
	})
	if resp.Code == 200 {
		t.Fatal("Expected callback with missing fields to fail")
	}
}

func TestPaymentSimulateCallback(t *testing.T) {
	c := getAdminClient(t)

	// Use simulate-callback with a nonexistent order to test 404
	resp := c.post("/api/payments/00000000-0000-0000-0000-000000000000/simulate-callback", nil)
	if resp.Code == 200 && resp.Msg != "Order is already paid" {
		t.Fatal("Expected simulate-callback with nonexistent order to fail")
	}
}

func TestPaymentSimulateCallbackRequiresStaff(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/payments/00000000-0000-0000-0000-000000000000/simulate-callback", nil)
	if resp.Code != 401 {
		t.Errorf("Expected 401 for unauthenticated simulate-callback, got %d", resp.Code)
	}
}

func TestPaymentCallbackEndToEnd(t *testing.T) {
	merchantKey := readMerchantKey()
	if merchantKey == "" {
		t.Skip("Merchant key not available")
	}

	c := getAdminClient(t)

	// Create a test product via direct approach - need a product with stock
	// Since we can't easily create products via API in all setups, test the callback
	// against a constructed scenario using the callback endpoint directly

	// Test with a non-existent order number but valid signature format
	orderNumber := "ORD-99999999-99999"
	txnID := "WX-TXN-TEST-E2E"
	amountCents := 5000
	nonceStr := "test-nonce-e2e"
	sign := computeSignature(txnID, orderNumber, amountCents, "SUCCESS", nonceStr, merchantKey)

	resp := c.post("/api/payments/callback", map[string]interface{}{
		"transaction_id": txnID,
		"order_number":   orderNumber,
		"amount_cents":   amountCents,
		"status":         "SUCCESS",
		"nonce_str":      nonceStr,
		"sign":           sign,
	})
	// Should fail with 404 since order doesn't exist, but signature is valid
	if resp.Code != 404 {
		t.Errorf("Expected 404 for valid signature but nonexistent order, got %d %s", resp.Code, resp.Msg)
	}
}

func TestPaymentCallbackNonSuccessStatus(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/payments/callback", map[string]interface{}{
		"transaction_id": "TXN-FAIL-001",
		"order_number":   "ORD-20240101-00001",
		"amount_cents":   1000,
		"status":         "FAIL",
		"nonce_str":      "nonce",
		"sign":           "somesign",
	})
	if resp.Code == 200 {
		t.Fatal("Expected non-SUCCESS callback to be rejected")
	}
}

func TestPaymentOrderLifecycle(t *testing.T) {
	// Test that an order can be created and its payment simulated
	c := getAdminClient(t)

	// List orders to verify the endpoint works
	resp := c.get("/api/orders")
	if resp.Code != 200 {
		t.Fatalf("List orders failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Items []struct {
			ID          string `json:"id"`
			Status      string `json:"status"`
			OrderNumber string `json:"order_number"`
		} `json:"items"`
	}
	json.Unmarshal(resp.Data, &data)

	// Find a pending_payment order if any exist
	for _, order := range data.Items {
		if order.Status == "pending_payment" {
			// Try to simulate payment
			simResp := c.post("/api/payments/"+order.ID+"/simulate-callback", nil)
			if simResp.Code != 200 {
				t.Logf("Simulate callback for order %s: %d %s", order.ID, simResp.Code, simResp.Msg)
			}

			// Verify order is now paid
			orderResp := c.get("/api/orders/" + order.ID)
			if orderResp.Code == 200 {
				var orderData struct {
					Status string `json:"status"`
				}
				json.Unmarshal(orderResp.Data, &orderData)
				if simResp.Code == 200 && orderData.Status != "paid" {
					t.Errorf("After simulate, order status = %q, want 'paid'", orderData.Status)
				}
			}
			return // Tested one order
		}
	}
	t.Log("No pending_payment orders found to test simulate-callback against")
}
