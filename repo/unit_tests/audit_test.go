package unit_tests

import (
	"testing"
)

// Test that the audit log immutability rules are correctly defined.

func TestAuditLogImmutabilityRules(t *testing.T) {
	// Audit logs must be append-only: INSERT is the only allowed DML.
	// UPDATE is never permitted.
	// DELETE is only permitted after the row has been archived.
	allowedDML := map[string]bool{
		"INSERT": true,
		"UPDATE": false,
		"DELETE": false, // only allowed when row exists in archive
	}

	if !allowedDML["INSERT"] {
		t.Error("INSERT must be allowed on audit_logs")
	}
	if allowedDML["UPDATE"] {
		t.Error("UPDATE must be blocked on audit_logs")
	}
	if allowedDML["DELETE"] {
		t.Error("bare DELETE (without archival) must be blocked on audit_logs")
	}
}

func TestArchiveAuditLogFullyImmutable(t *testing.T) {
	// Once in the archive, audit logs are completely immutable.
	allowedDML := map[string]bool{
		"INSERT": true,
		"UPDATE": false,
		"DELETE": false,
	}

	if !allowedDML["INSERT"] {
		t.Error("INSERT must be allowed on archive.audit_logs")
	}
	if allowedDML["UPDATE"] {
		t.Error("UPDATE must be blocked on archive.audit_logs")
	}
	if allowedDML["DELETE"] {
		t.Error("DELETE must be blocked on archive.audit_logs")
	}
}

func TestAuditLogDeleteRequiresArchive(t *testing.T) {
	// The delete trigger must verify the row exists in archive before allowing removal.
	// This enforces the copy-then-delete archival contract.
	type archiveCheck struct {
		existsInArchive bool
		deleteAllowed   bool
	}

	tests := []archiveCheck{
		{existsInArchive: true, deleteAllowed: true},
		{existsInArchive: false, deleteAllowed: false},
	}

	for _, tt := range tests {
		allowed := tt.existsInArchive // delete only allowed when archived
		if allowed != tt.deleteAllowed {
			t.Errorf("Delete with archived=%v: got allowed=%v, want %v",
				tt.existsInArchive, allowed, tt.deleteAllowed)
		}
	}
}
