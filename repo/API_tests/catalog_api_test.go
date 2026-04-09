//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

func TestCatalogEndpoint(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/catalog")
	if resp.Code != 200 {
		t.Fatalf("Catalog failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Items []interface{} `json:"items"`
		Total int           `json:"total"`
	}
	json.Unmarshal(resp.Data, &data)
	// May be empty initially, just ensure it doesn't error
}

func TestCatalogWithTypeFilter(t *testing.T) {
	c := getAdminClient(t)

	// Filter sessions
	resp := c.get("/api/catalog?type=session")
	if resp.Code != 200 {
		t.Fatalf("Catalog session filter failed: %d %s", resp.Code, resp.Msg)
	}

	// Filter products
	resp = c.get("/api/catalog?type=product")
	if resp.Code != 200 {
		t.Fatalf("Catalog product filter failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestCatalogSearch(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/catalog?q=nonexistent")
	if resp.Code != 200 {
		t.Fatalf("Catalog search failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestCatalogPagination(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/catalog?page=1&page_size=5")
	if resp.Code != 200 {
		t.Fatalf("Catalog pagination failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.Page != 1 {
		t.Errorf("Page = %d, want 1", data.Page)
	}
	if data.PageSize != 5 {
		t.Errorf("PageSize = %d, want 5", data.PageSize)
	}
}

func TestCatalogUnauthenticated(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/catalog")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
