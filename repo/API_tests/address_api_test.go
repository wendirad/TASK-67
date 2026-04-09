//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

func TestAddressCRUD(t *testing.T) {
	c := getAdminClient(t)

	// Create address
	createResp := c.post("/api/addresses", map[string]interface{}{
		"label":          "Home",
		"recipient_name": "Test Recipient",
		"phone":          "13800138000",
		"address_line1":  "123 Test Street",
		"city":           "Beijing",
		"province":       "Beijing",
		"postal_code":    "100000",
	})
	if createResp.Code != 201 {
		t.Fatalf("Create address failed: %d %s", createResp.Code, createResp.Msg)
	}

	var addr struct {
		ID string `json:"id"`
	}
	json.Unmarshal(createResp.Data, &addr)
	if addr.ID == "" {
		t.Fatal("Address ID is empty")
	}

	// List addresses
	listResp := c.get("/api/addresses")
	if listResp.Code != 200 {
		t.Fatalf("List addresses failed: %d %s", listResp.Code, listResp.Msg)
	}

	// Update address
	updateResp := c.put("/api/addresses/"+addr.ID, map[string]interface{}{
		"label":          "Office",
		"recipient_name": "Updated Recipient",
		"phone":          "13800138000",
		"address_line1":  "456 Updated Street",
		"city":           "Shanghai",
		"province":       "Shanghai",
		"postal_code":    "200000",
	})
	if updateResp.Code != 200 {
		t.Fatalf("Update address failed: %d %s", updateResp.Code, updateResp.Msg)
	}

	// Set default
	defaultResp := c.put("/api/addresses/"+addr.ID+"/default", nil)
	if defaultResp.Code != 200 {
		t.Fatalf("Set default failed: %d %s", defaultResp.Code, defaultResp.Msg)
	}

	// Delete
	deleteResp := c.delete("/api/addresses/" + addr.ID)
	if deleteResp.Code != 200 {
		t.Fatalf("Delete address failed: %d %s", deleteResp.Code, deleteResp.Msg)
	}
}

func TestAddressRequiresAuth(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/addresses")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
