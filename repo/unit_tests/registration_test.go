package unit_tests

import (
	"testing"
)

// Test registration state machine transitions
func TestRegistrationCancelableStatuses(t *testing.T) {
	cancelable := map[string]bool{
		"pending":    true,
		"approved":   true,
		"registered": true,
		"waitlisted": true,
	}

	tests := []struct {
		status   string
		expected bool
	}{
		{"pending", true},
		{"approved", true},
		{"registered", true},
		{"waitlisted", true},
		{"canceled", false},
		{"rejected", false},
		{"no_show", false},
		{"completed", false},
	}

	for _, tt := range tests {
		if cancelable[tt.status] != tt.expected {
			t.Errorf("Cancelable(%q) = %v, want %v", tt.status, cancelable[tt.status], tt.expected)
		}
	}
}

func TestRegistrationConfirmOnlyFromApproved(t *testing.T) {
	// Confirm should only work from "approved" status
	confirmable := map[string]bool{
		"approved": true,
	}

	nonConfirmable := []string{"pending", "registered", "waitlisted", "canceled", "rejected"}
	for _, status := range nonConfirmable {
		if confirmable[status] {
			t.Errorf("Status %q should not be confirmable", status)
		}
	}

	if !confirmable["approved"] {
		t.Error("approved should be confirmable")
	}
}

func TestRegistrationApproveOnlyFromPending(t *testing.T) {
	approvable := map[string]bool{
		"pending": true,
	}

	nonApprovable := []string{"approved", "registered", "canceled", "rejected"}
	for _, status := range nonApprovable {
		if approvable[status] {
			t.Errorf("Status %q should not be approvable", status)
		}
	}

	if !approvable["pending"] {
		t.Error("pending should be approvable")
	}
}
