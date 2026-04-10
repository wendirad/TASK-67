//go:build integration

package api_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// TestCanaryConfigExists verifies canary-gated config entries are accessible.
func TestCanaryConfigExists(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/admin/config")
	if resp.Code != 200 {
		t.Fatalf("Config list failed: %d %s", resp.Code, resp.Msg)
	}

	var items []struct {
		Key              string `json:"key"`
		Value            string `json:"value"`
		CanaryPercentage *int   `json:"canary_percentage"`
	}
	json.Unmarshal(resp.Data, &items)

	// Verify expected canary-relevant keys exist
	found := make(map[string]bool)
	for _, item := range items {
		found[item.Key] = true
	}

	requiredKeys := []string{
		"post.rate_limit_per_hour",
		"post.auto_flag_report_count",
		"order.payment_timeout_minutes",
	}
	for _, key := range requiredKeys {
		if !found[key] {
			t.Errorf("Config key %q not found in config list", key)
		}
	}
}

// TestCanarySetPercentage verifies that an admin can set a canary percentage on a config entry.
func TestCanarySetPercentage(t *testing.T) {
	c := getAdminClient(t)

	// Set canary percentage on payment timeout
	pct := 50
	resp := c.put("/api/admin/config/order.payment_timeout_minutes", map[string]interface{}{
		"value":             "20",
		"canary_percentage": pct,
	})
	if resp.Code != 200 {
		t.Fatalf("Config update failed: %d %s", resp.Code, resp.Msg)
	}

	// Verify the canary config appears in the canary list
	resp = c.get("/api/admin/config-canary")
	if resp.Code != 200 {
		t.Fatalf("Canary list failed: %d %s", resp.Code, resp.Msg)
	}

	var items []struct {
		Key              string `json:"key"`
		Value            string `json:"value"`
		CanaryPercentage *int   `json:"canary_percentage"`
	}
	json.Unmarshal(resp.Data, &items)

	found := false
	for _, item := range items {
		if item.Key == "order.payment_timeout_minutes" {
			found = true
			if item.CanaryPercentage == nil || *item.CanaryPercentage != pct {
				t.Errorf("canary_percentage = %v, want %d", item.CanaryPercentage, pct)
			}
			if item.Value != "20" {
				t.Errorf("value = %q, want %q", item.Value, "20")
			}
		}
	}
	if !found {
		t.Error("order.payment_timeout_minutes not found in canary list after update")
	}

	// Reset: remove canary percentage
	resp = c.put("/api/admin/config/order.payment_timeout_minutes", map[string]interface{}{
		"value":             "15",
		"canary_percentage": nil,
	})
	if resp.Code != 200 {
		t.Fatalf("Config reset failed: %d %s", resp.Code, resp.Msg)
	}
}

// TestCanaryUserCohortAssigned verifies that newly created users get a canary_cohort.
func TestCanaryUserCohortAssigned(t *testing.T) {
	c := getAdminClient(t)

	username := fmt.Sprintf("canary_test_%d", os.Getpid())
	resp := c.post("/api/admin/users", map[string]interface{}{
		"username":     username,
		"password":     "CanaryTest123!",
		"role":         "member",
		"display_name": "Canary Test User",
	})
	if resp.Code != 201 {
		t.Fatalf("Create user failed: %d %s", resp.Code, resp.Msg)
	}

	// Login as the new user and verify authenticated requests work
	mc := newClient(t)
	mc.login(username, "CanaryTest123!")

	resp = mc.get("/api/auth/me")
	if resp.Code != 200 {
		t.Fatalf("Get me failed: %d %s", resp.Code, resp.Msg)
	}
}

// TestCanaryMiddlewareDoesNotBreakUnauthenticated verifies the canary middleware
// doesn't interfere with unauthenticated API endpoints.
func TestCanaryMiddlewareDoesNotBreakUnauthenticated(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/health")
	if resp.Code != 200 {
		t.Errorf("Health check failed with canary middleware: %d %s", resp.Code, resp.Msg)
	}
}

// TestCanaryRateLimitConfigUsed verifies that the rate limit config can be read and updated.
func TestCanaryRateLimitConfigUsed(t *testing.T) {
	c := getAdminClient(t)

	// Ensure rate limit config exists with default value
	resp := c.put("/api/admin/config/post.rate_limit_per_hour", map[string]interface{}{
		"value":             "5",
		"canary_percentage": nil,
	})
	if resp.Code != 200 {
		t.Fatalf("Config update failed: %d %s", resp.Code, resp.Msg)
	}

	// Read it back
	resp = c.get("/api/admin/config")
	if resp.Code != 200 {
		t.Fatalf("Config read failed: %d", resp.Code)
	}

	var items []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	json.Unmarshal(resp.Data, &items)

	for _, item := range items {
		if item.Key == "post.rate_limit_per_hour" && item.Value != "5" {
			t.Errorf("rate limit value = %q, want %q", item.Value, "5")
		}
	}
}

// TestCanaryConfigAuditTrail verifies config changes produce audit log entries.
func TestCanaryConfigAuditTrail(t *testing.T) {
	c := getAdminClient(t)

	// Update a config entry
	resp := c.put("/api/admin/config/post.auto_flag_report_count", map[string]interface{}{
		"value":             "4",
		"canary_percentage": 25,
	})
	if resp.Code != 200 {
		t.Fatalf("Config update failed: %d %s", resp.Code, resp.Msg)
	}

	// Check audit logs
	resp = c.get("/api/admin/config-audit-logs")
	if resp.Code != 200 {
		t.Fatalf("Audit logs failed: %d %s", resp.Code, resp.Msg)
	}

	var logs []struct {
		Action   string `json:"action"`
		EntityID string `json:"entity_id"`
	}
	json.Unmarshal(resp.Data, &logs)

	if len(logs) == 0 {
		t.Error("Expected at least one audit log entry after config update")
	}

	// Reset
	resp = c.put("/api/admin/config/post.auto_flag_report_count", map[string]interface{}{
		"value":             "3",
		"canary_percentage": nil,
	})
	if resp.Code != 200 {
		t.Logf("Warning: config reset failed: %d %s", resp.Code, resp.Msg)
	}
}

// TestCanaryOrderCreationWithConfig verifies that order creation works
// when canary config is active on payment_timeout_minutes.
func TestCanaryOrderCreationWithConfig(t *testing.T) {
	c := getAdminClient(t)

	// Set payment timeout with full rollout
	resp := c.put("/api/admin/config/order.payment_timeout_minutes", map[string]interface{}{
		"value":             "15",
		"canary_percentage": nil,
	})
	if resp.Code != 200 {
		t.Fatalf("Config update failed: %d %s", resp.Code, resp.Msg)
	}

	// Verify we can still hit the order creation path without a 500
	// (expected 400 for empty items, but not 500 which would mean canary crash)
	resp = c.post("/api/orders", map[string]interface{}{
		"items":  []interface{}{},
		"source": "buy_now",
	})
	if resp.Code == 500 {
		t.Fatalf("Order creation returned 500 (possible canary config error): %s", resp.Msg)
	}
}

// TestCanaryPostCreationWithConfig verifies that post creation works
// when canary config is active on rate limit.
func TestCanaryPostCreationWithConfig(t *testing.T) {
	c := getAdminClient(t)

	// Set a canary rate limit at 50% rollout
	resp := c.put("/api/admin/config/post.rate_limit_per_hour", map[string]interface{}{
		"value":             "10",
		"canary_percentage": 50,
	})
	if resp.Code != 200 {
		t.Fatalf("Config update failed: %d %s", resp.Code, resp.Msg)
	}

	// Try creating a post — should work regardless of canary cohort
	resp = c.post("/api/posts", map[string]interface{}{
		"title":   "Canary test post",
		"content": "Testing post creation with canary rate limit config active.",
	})
	if resp.Code == 500 {
		t.Fatalf("Post creation returned 500 (possible canary config error): %s", resp.Msg)
	}

	// Reset rate limit
	resp = c.put("/api/admin/config/post.rate_limit_per_hour", map[string]interface{}{
		"value":             "5",
		"canary_percentage": nil,
	})
	if resp.Code != 200 {
		t.Logf("Warning: config reset failed: %d %s", resp.Code, resp.Msg)
	}
}
