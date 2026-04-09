//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/health")
	if resp.Code != 200 {
		t.Fatalf("Health check failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Status   string `json:"status"`
		Database string `json:"database"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.Status != "healthy" {
		t.Errorf("Status = %q, want healthy", data.Status)
	}
	if data.Database != "connected" {
		t.Errorf("Database = %q, want connected", data.Database)
	}
}

func TestLoginSuccess(t *testing.T) {
	password := readAdminPassword()
	if password == "" {
		t.Skip("Admin password not available")
	}

	c := newClient(t)
	resp := c.post("/api/auth/login", map[string]string{
		"username": "admin",
		"password": password,
	})
	if resp.Code != 200 {
		t.Fatalf("Login failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Token string `json:"token"`
		User  struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"user"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.Token == "" {
		t.Error("Token is empty")
	}
	if data.User.Username != "admin" {
		t.Errorf("Username = %q, want admin", data.User.Username)
	}
	if data.User.Role != "admin" {
		t.Errorf("Role = %q, want admin", data.User.Role)
	}
}

func TestLoginFailure(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/auth/login", map[string]string{
		"username": "admin",
		"password": "wrongpassword",
	})
	if resp.Code == 200 {
		t.Fatal("Expected login to fail with wrong password")
	}
}

func TestLoginMissingFields(t *testing.T) {
	c := newClient(t)
	resp := c.post("/api/auth/login", map[string]string{
		"username": "",
		"password": "",
	})
	if resp.Code == 200 {
		t.Fatal("Expected login to fail with empty fields")
	}
}

func TestMeEndpoint(t *testing.T) {
	password := readAdminPassword()
	if password == "" {
		t.Skip("Admin password not available")
	}

	c := newClient(t)
	c.login("admin", password)

	resp := c.get("/api/auth/me")
	if resp.Code != 200 {
		t.Fatalf("Me endpoint failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.Username != "admin" {
		t.Errorf("Username = %q, want admin", data.Username)
	}
}

func TestMeWithoutAuth(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/auth/me")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}

func TestLogout(t *testing.T) {
	password := readAdminPassword()
	if password == "" {
		t.Skip("Admin password not available")
	}

	c := newClient(t)
	c.login("admin", password)

	resp := c.post("/api/auth/logout", nil)
	if resp.Code != 200 {
		t.Fatalf("Logout failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestChangePasswordValidation(t *testing.T) {
	password := readAdminPassword()
	if password == "" {
		t.Skip("Admin password not available")
	}

	c := newClient(t)
	c.login("admin", password)

	// Try changing to a weak password
	resp := c.post("/api/auth/change-password", map[string]string{
		"current_password": password,
		"new_password":     "weak",
	})
	if resp.Code == 200 {
		t.Fatal("Expected password change to fail with weak password")
	}
}

func TestRBACAdminEndpoint(t *testing.T) {
	// Unauthenticated access to admin endpoint
	c := newClient(t)
	resp := c.get("/api/admin/users")
	if resp.Code != 401 {
		t.Errorf("Expected 401 for unauthenticated admin access, got %d", resp.Code)
	}
}
