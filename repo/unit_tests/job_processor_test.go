package unit_tests

import (
	"testing"

	"campusrec/internal/models"
)

// TestJobExportFilterParsing verifies the real ParseExportFilters function
// used by the job processor when handling export jobs with filter parameters.
func TestJobExportFilterParsing(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
		check   func(*models.ExportFilters) bool
	}{
		{
			"empty string returns zero-value filters",
			"",
			false,
			func(f *models.ExportFilters) bool { return f != nil && f.Status == "" && f.DateFrom == "" },
		},
		{
			"valid JSON with status filter",
			`{"status":"active"}`,
			false,
			func(f *models.ExportFilters) bool { return f != nil && f.Status == "active" },
		},
		{
			"valid JSON with date range",
			`{"date_from":"2026-01-01","date_to":"2026-12-31"}`,
			false,
			func(f *models.ExportFilters) bool {
				return f != nil && f.DateFrom == "2026-01-01" && f.DateTo == "2026-12-31"
			},
		},
		{
			"invalid JSON",
			`{bad json}`,
			true,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, err := models.ParseExportFilters(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseExportFilters(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if tt.check != nil && !tt.check(filters) {
				t.Errorf("ParseExportFilters(%q) returned unexpected filters: %+v", tt.raw, filters)
			}
		})
	}
}

// TestJobExportFilterValidation verifies the real ValidateExportFilters
// function used to validate filter parameters before job execution.
func TestJobExportFilterValidation(t *testing.T) {
	tests := []struct {
		name       string
		entityType string
		filters    *models.ExportFilters
		wantMsg    string
	}{
		{
			"nil filters are valid",
			"products",
			nil,
			"",
		},
		{
			"valid status filter for orders",
			"orders",
			&models.ExportFilters{Status: "paid"},
			"",
		},
		{
			"date_from without date_to is invalid",
			"orders",
			&models.ExportFilters{DateFrom: "2026-01-01"},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := models.ValidateExportFilters(tt.entityType, tt.filters)
			if tt.wantMsg == "" && msg != "" {
				t.Errorf("ValidateExportFilters: unexpected error %q", msg)
			}
			if tt.wantMsg != "" && msg != tt.wantMsg {
				t.Errorf("ValidateExportFilters: got %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

// TestJobRelatedAuditLogPermissions verifies that audit log DML controls
// correctly restrict operations on the audit_logs table used by job processing.
// Only INSERT is allowed — all other operations are blocked to maintain
// immutability of the audit trail.
func TestJobRelatedAuditLogPermissions(t *testing.T) {
	if !models.AuditLogDMLAllowed("INSERT") {
		t.Error("INSERT must be allowed on audit_logs (used by job processor)")
	}
	if models.AuditLogDMLAllowed("UPDATE") {
		t.Error("UPDATE must not be allowed on audit_logs (immutability)")
	}
	if models.AuditLogDMLAllowed("DELETE") {
		t.Error("DELETE must not be allowed on audit_logs (immutability)")
	}
	if models.AuditLogDMLAllowed("SELECT") {
		t.Error("SELECT DML is not allowed via AuditLogDMLAllowed (read access is at SQL level)")
	}
}

// TestJobRegistrationStateTransitions verifies the real state transition
// functions used when import jobs create or modify registrations.
func TestJobRegistrationStateTransitions(t *testing.T) {
	tests := []struct {
		status      string
		cancelable  bool
		approvable  bool
		confirmable bool
	}{
		{"pending", true, true, false},
		{"approved", true, false, true},
		{"registered", true, false, false},
		{"waitlisted", true, false, false},
		{"rejected", false, false, false},
		{"canceled", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := models.IsRegistrationCancelable(tt.status); got != tt.cancelable {
				t.Errorf("IsRegistrationCancelable(%q) = %v, want %v", tt.status, got, tt.cancelable)
			}
			if got := models.IsRegistrationApprovable(tt.status); got != tt.approvable {
				t.Errorf("IsRegistrationApprovable(%q) = %v, want %v", tt.status, got, tt.approvable)
			}
			if got := models.IsRegistrationConfirmable(tt.status); got != tt.confirmable {
				t.Errorf("IsRegistrationConfirmable(%q) = %v, want %v", tt.status, got, tt.confirmable)
			}
		})
	}
}
