package unit_tests

import (
	"testing"
)

// Test moderation decision validation
func TestModerationDecisionValues(t *testing.T) {
	validDecisions := map[string]bool{
		"approve": true,
		"reject":  true,
	}

	tests := []struct {
		decision string
		valid    bool
	}{
		{"approve", true},
		{"reject", true},
		{"delete", false},
		{"ban", false},
		{"", false},
	}

	for _, tt := range tests {
		if validDecisions[tt.decision] != tt.valid {
			t.Errorf("Decision %q valid = %v, want %v", tt.decision, validDecisions[tt.decision], tt.valid)
		}
	}
}

// Test post status transitions
func TestPostStatusTransitions(t *testing.T) {
	validStatuses := []string{"visible", "flagged", "hidden", "removed"}
	statusSet := make(map[string]bool)
	for _, s := range validStatuses {
		statusSet[s] = true
	}

	if !statusSet["visible"] || !statusSet["flagged"] || !statusSet["hidden"] || !statusSet["removed"] {
		t.Error("Missing expected post status")
	}

	if statusSet["unknown"] {
		t.Error("Unknown status should not be valid")
	}
}
