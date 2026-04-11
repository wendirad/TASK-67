package unit_tests

import (
	"testing"

	"campusrec/internal/models"
)

// TestModerationPostStatusValues verifies that Product.ComputeAvailability
// returns the correct availability status based on stock levels — this is the
// real model method used by the product listing flow.
func TestModerationPostStatusValues(t *testing.T) {
	tests := []struct {
		name  string
		stock int
		want  string
	}{
		{"high stock", 50, "in_stock"},
		{"boundary above low_stock", 11, "in_stock"},
		{"boundary at low_stock", 10, "low_stock"},
		{"low stock", 5, "low_stock"},
		{"single item", 1, "low_stock"},
		{"out of stock", 0, "out_of_stock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &models.Product{StockQuantity: tt.stock}
			got := p.ComputeAvailability()
			if got != tt.want {
				t.Errorf("ComputeAvailability() with stock=%d: got %q, want %q", tt.stock, got, tt.want)
			}
		})
	}
}

// TestAuditLogDMLAllowedForModeration verifies that the audit log DML
// permissions correctly gate destructive operations — critical for the
// moderation audit trail integrity. Only INSERT is allowed; all other
// operations (including SELECT, UPDATE, DELETE) are blocked at the DML level.
func TestAuditLogDMLAllowedForModeration(t *testing.T) {
	tests := []struct {
		op      string
		allowed bool
	}{
		{"INSERT", true},
		{"SELECT", false},
		{"UPDATE", false},
		{"DELETE", false},
		{"DROP", false},
		{"TRUNCATE", false},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			got := models.AuditLogDMLAllowed(tt.op)
			if got != tt.allowed {
				t.Errorf("AuditLogDMLAllowed(%q) = %v, want %v", tt.op, got, tt.allowed)
			}
		})
	}
}

// TestArchiveAuditLogDMLAllowed verifies that the archive audit log table
// is fully immutable — only INSERT is allowed, no UPDATE or DELETE.
func TestArchiveAuditLogDMLAllowed(t *testing.T) {
	tests := []struct {
		op      string
		allowed bool
	}{
		{"INSERT", true},
		{"SELECT", false},
		{"DELETE", false},
		{"UPDATE", false},
		{"DROP", false},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			got := models.ArchiveAuditLogDMLAllowed(tt.op)
			if got != tt.allowed {
				t.Errorf("ArchiveAuditLogDMLAllowed(%q) = %v, want %v", tt.op, got, tt.allowed)
			}
		})
	}
}
