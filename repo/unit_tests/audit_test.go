package unit_tests

import (
	"testing"

	"campusrec/internal/models"
)

// TestAuditLogImmutabilityRules calls the real models.AuditLogDMLAllowed
// function to verify audit log immutability rules enforced at the DB level.
func TestAuditLogImmutabilityRules(t *testing.T) {
	tests := []struct {
		op      string
		allowed bool
	}{
		{"INSERT", true},
		{"UPDATE", false},
		{"DELETE", false},
	}

	for _, tt := range tests {
		got := models.AuditLogDMLAllowed(tt.op)
		if got != tt.allowed {
			t.Errorf("AuditLogDMLAllowed(%q) = %v, want %v", tt.op, got, tt.allowed)
		}
	}
}

// TestArchiveAuditLogFullyImmutable calls the real
// models.ArchiveAuditLogDMLAllowed function to verify that archive audit logs
// are fully immutable (only INSERT allowed).
func TestArchiveAuditLogFullyImmutable(t *testing.T) {
	tests := []struct {
		op      string
		allowed bool
	}{
		{"INSERT", true},
		{"UPDATE", false},
		{"DELETE", false},
	}

	for _, tt := range tests {
		got := models.ArchiveAuditLogDMLAllowed(tt.op)
		if got != tt.allowed {
			t.Errorf("ArchiveAuditLogDMLAllowed(%q) = %v, want %v", tt.op, got, tt.allowed)
		}
	}
}

// TestAuditLogDeleteRequiresArchive calls the real
// models.AuditLogDeleteRequiresArchive function to verify the archival
// precondition for deletion.
func TestAuditLogDeleteRequiresArchive(t *testing.T) {
	if !models.AuditLogDeleteRequiresArchive() {
		t.Error("AuditLogDeleteRequiresArchive() should return true")
	}
}

// TestAuditLogUnknownOpBlocked verifies that any DML operation other than
// INSERT is blocked by the production function.
func TestAuditLogUnknownOpBlocked(t *testing.T) {
	unknownOps := []string{"TRUNCATE", "MERGE", "UPSERT", ""}
	for _, op := range unknownOps {
		if models.AuditLogDMLAllowed(op) {
			t.Errorf("AuditLogDMLAllowed(%q) should be false for unknown operations", op)
		}
		if models.ArchiveAuditLogDMLAllowed(op) {
			t.Errorf("ArchiveAuditLogDMLAllowed(%q) should be false for unknown operations", op)
		}
	}
}
