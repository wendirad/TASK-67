package unit_tests

import (
	"testing"

	"campusrec/internal/models"
)

func TestParseExportFiltersEmpty(t *testing.T) {
	f, err := models.ParseExportFilters("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Status != "" || f.Role != "" || f.Category != "" {
		t.Error("empty string should produce zero-value filters")
	}
}

func TestParseExportFiltersValid(t *testing.T) {
	f, err := models.ParseExportFilters(`{"status":"active","role":"member","date_from":"2026-01-01"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Status != "active" {
		t.Errorf("status = %q, want %q", f.Status, "active")
	}
	if f.Role != "member" {
		t.Errorf("role = %q, want %q", f.Role, "member")
	}
	if f.DateFrom != "2026-01-01" {
		t.Errorf("date_from = %q, want %q", f.DateFrom, "2026-01-01")
	}
}

func TestParseExportFiltersInvalidJSON(t *testing.T) {
	_, err := models.ParseExportFilters(`{bad json}`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateExportFiltersStatusPerEntity(t *testing.T) {
	tests := []struct {
		entity string
		status string
		valid  bool
	}{
		{"users", "active", true},
		{"users", "banned", true},
		{"users", "nonexistent", false},
		{"products", "active", true},
		{"products", "out_of_stock", true},
		{"products", "banned", false},
		{"sessions", "open", true},
		{"sessions", "canceled", true},
		{"sessions", "active", false},
		{"orders", "paid", true},
		{"orders", "refunded", true},
		{"orders", "open", false},
		{"registrations", "pending", true},
		{"registrations", "no_show", true},
		{"registrations", "paid", false},
		{"tickets", "open", true},
		{"tickets", "resolved", true},
		{"tickets", "active", false},
	}

	for _, tt := range tests {
		f := &models.ExportFilters{Status: tt.status}
		msg := models.ValidateExportFilters(tt.entity, f)
		if tt.valid && msg != "" {
			t.Errorf("ValidateExportFilters(%q, status=%q) unexpected error: %s", tt.entity, tt.status, msg)
		}
		if !tt.valid && msg == "" {
			t.Errorf("ValidateExportFilters(%q, status=%q) expected error but got none", tt.entity, tt.status)
		}
	}
}

func TestValidateExportFiltersRoleOnlyForUsers(t *testing.T) {
	f := &models.ExportFilters{Role: "member"}

	if msg := models.ValidateExportFilters("users", f); msg != "" {
		t.Errorf("role=member for users should be valid, got: %s", msg)
	}

	if msg := models.ValidateExportFilters("orders", f); msg == "" {
		t.Error("role filter for orders should be invalid")
	}
}

func TestValidateExportFiltersInvalidRole(t *testing.T) {
	f := &models.ExportFilters{Role: "superadmin"}
	if msg := models.ValidateExportFilters("users", f); msg == "" {
		t.Error("role=superadmin should be invalid")
	}
}

func TestValidateExportFiltersCategoryOnlyForProducts(t *testing.T) {
	f := &models.ExportFilters{Category: "Accessories"}

	if msg := models.ValidateExportFilters("products", f); msg != "" {
		t.Errorf("category for products should be valid, got: %s", msg)
	}

	if msg := models.ValidateExportFilters("users", f); msg == "" {
		t.Error("category filter for users should be invalid")
	}
}

func TestValidateExportFiltersTicketSpecific(t *testing.T) {
	// Type filter only for tickets
	f := &models.ExportFilters{Type: "general"}
	if msg := models.ValidateExportFilters("tickets", f); msg != "" {
		t.Errorf("type=general for tickets should be valid, got: %s", msg)
	}
	if msg := models.ValidateExportFilters("orders", f); msg == "" {
		t.Error("type filter for orders should be invalid")
	}

	// Priority filter only for tickets
	f = &models.ExportFilters{Priority: "high"}
	if msg := models.ValidateExportFilters("tickets", f); msg != "" {
		t.Errorf("priority=high for tickets should be valid, got: %s", msg)
	}
	if msg := models.ValidateExportFilters("sessions", f); msg == "" {
		t.Error("priority filter for sessions should be invalid")
	}
}

func TestValidateExportFiltersInvalidTicketValues(t *testing.T) {
	f := &models.ExportFilters{Type: "nonexistent"}
	if msg := models.ValidateExportFilters("tickets", f); msg == "" {
		t.Error("invalid ticket type should be rejected")
	}

	f = &models.ExportFilters{Priority: "urgent"}
	if msg := models.ValidateExportFilters("tickets", f); msg == "" {
		t.Error("invalid priority should be rejected")
	}
}

func TestValidateExportFiltersDateFormat(t *testing.T) {
	f := &models.ExportFilters{DateFrom: "2026-01-01"}
	if msg := models.ValidateExportFilters("users", f); msg != "" {
		t.Errorf("valid date_from should pass, got: %s", msg)
	}

	f = &models.ExportFilters{DateFrom: "01/01/2026"}
	if msg := models.ValidateExportFilters("users", f); msg == "" {
		t.Error("invalid date_from format should be rejected")
	}

	f = &models.ExportFilters{DateTo: "not-a-date"}
	if msg := models.ValidateExportFilters("users", f); msg == "" {
		t.Error("invalid date_to format should be rejected")
	}
}

func TestValidateExportFiltersNilFilters(t *testing.T) {
	if msg := models.ValidateExportFilters("users", nil); msg != "" {
		t.Errorf("nil filters should be valid, got: %s", msg)
	}
}

func TestValidateExportFiltersCombined(t *testing.T) {
	f := &models.ExportFilters{
		Status:   "active",
		Role:     "member",
		DateFrom: "2026-01-01",
		DateTo:   "2026-12-31",
	}
	if msg := models.ValidateExportFilters("users", f); msg != "" {
		t.Errorf("combined valid filters should pass, got: %s", msg)
	}
}
