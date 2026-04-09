//go:build integration

package api_tests

import (
	"testing"
)

func TestKPIOverview(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/kpi/overview")
	if resp.Code != 200 {
		t.Fatalf("KPI overview failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestKPIFillRate(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/kpi/fill-rate?from_date=2024-01-01&to_date=2024-12-31")
	if resp.Code != 200 {
		t.Fatalf("KPI fill rate failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestKPIMembers(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/kpi/members?from_date=2024-01-01&to_date=2024-12-31")
	if resp.Code != 200 {
		t.Fatalf("KPI members failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestKPIEngagement(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/kpi/engagement?from_date=2024-01-01&to_date=2024-12-31")
	if resp.Code != 200 {
		t.Fatalf("KPI engagement failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestKPICoaches(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/kpi/coaches?from_date=2024-01-01&to_date=2024-12-31")
	if resp.Code != 200 {
		t.Fatalf("KPI coaches failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestKPIRevenue(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/kpi/revenue?from_date=2024-01-01&to_date=2024-12-31")
	if resp.Code != 200 {
		t.Fatalf("KPI revenue failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestKPITickets(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/kpi/tickets?from_date=2024-01-01&to_date=2024-12-31")
	if resp.Code != 200 {
		t.Fatalf("KPI tickets failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestKPIRequiresAdmin(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/kpi/overview")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
