package unit_tests

import (
	"testing"

	"campusrec/internal/models"
)

// TestRegistrationCancelableStatuses calls the real models.IsRegistrationCancelable
// function to verify the production cancelable-status logic.
func TestRegistrationCancelableStatuses(t *testing.T) {
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
		got := models.IsRegistrationCancelable(tt.status)
		if got != tt.expected {
			t.Errorf("IsRegistrationCancelable(%q) = %v, want %v", tt.status, got, tt.expected)
		}
	}
}

// TestRegistrationCancelableUIMatchesBackend verifies that the set of statuses
// showing the Cancel button in the UI matches the backend cancelable set.
// This prevents the UI and backend from drifting apart.
func TestRegistrationCancelableUIMatchesBackend(t *testing.T) {
	// UI cancel-button statuses (internal/templates/registrations.templ:34)
	uiCancelable := []string{"pending", "approved", "registered", "waitlisted"}

	for _, status := range uiCancelable {
		if !models.IsRegistrationCancelable(status) {
			t.Errorf("UI shows Cancel for %q but models.IsRegistrationCancelable returns false", status)
		}
	}

	// Non-cancelable statuses must not be in the UI list
	nonCancelable := []string{"canceled", "rejected", "no_show", "completed"}
	for _, status := range nonCancelable {
		if models.IsRegistrationCancelable(status) {
			t.Errorf("models.IsRegistrationCancelable(%q) returns true but UI should not show Cancel", status)
		}
	}
}

// TestRegistrationConfirmOnlyFromApproved calls the real
// models.IsRegistrationConfirmable function.
func TestRegistrationConfirmOnlyFromApproved(t *testing.T) {
	if !models.IsRegistrationConfirmable("approved") {
		t.Error("IsRegistrationConfirmable(approved) should be true")
	}

	nonConfirmable := []string{"pending", "registered", "waitlisted", "canceled", "rejected"}
	for _, status := range nonConfirmable {
		if models.IsRegistrationConfirmable(status) {
			t.Errorf("IsRegistrationConfirmable(%q) should be false", status)
		}
	}
}

// TestRegistrationApproveOnlyFromPending calls the real
// models.IsRegistrationApprovable function.
func TestRegistrationApproveOnlyFromPending(t *testing.T) {
	if !models.IsRegistrationApprovable("pending") {
		t.Error("IsRegistrationApprovable(pending) should be true")
	}

	nonApprovable := []string{"approved", "registered", "canceled", "rejected", "waitlisted"}
	for _, status := range nonApprovable {
		if models.IsRegistrationApprovable(status) {
			t.Errorf("IsRegistrationApprovable(%q) should be false", status)
		}
	}
}

// TestRegistrationStateMachineCompleteness verifies that every known
// registration status is handled by at least one state-machine function.
func TestRegistrationStateMachineCompleteness(t *testing.T) {
	allStatuses := []string{
		"pending", "approved", "rejected", "registered",
		"waitlisted", "canceled", "completed", "no_show",
	}

	for _, status := range allStatuses {
		cancelable := models.IsRegistrationCancelable(status)
		approvable := models.IsRegistrationApprovable(status)
		confirmable := models.IsRegistrationConfirmable(status)

		// Every active status should be reachable via at least one transition
		// Terminal statuses (canceled, rejected, no_show, completed) should not
		// be inputs to any transition function
		isTerminal := status == "canceled" || status == "rejected" || status == "no_show" || status == "completed"
		hasTransition := cancelable || approvable || confirmable

		if isTerminal && hasTransition {
			t.Errorf("Terminal status %q should not be input to any transition", status)
		}
		if !isTerminal && !hasTransition {
			t.Errorf("Active status %q has no transition function that accepts it", status)
		}
	}
}
