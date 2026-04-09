//go:build integration

package api_tests

import (
	"testing"
)

func TestStaffShippingList(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/staff/shipping")
	if resp.Code != 200 {
		t.Fatalf("List shipping failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestShippingRequiresStaffRole(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/staff/shipping")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
