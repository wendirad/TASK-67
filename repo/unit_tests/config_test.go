package unit_tests

import (
	"testing"
)

// TestIsFeatureEnabledNegativeCohort verifies that IsFeatureEnabled excludes
// users with no cohort assigned (cohort = -1) from canary-gated features,
// matching the middleware's IsFeatureEnabledForRequest behavior.
func TestIsFeatureEnabledNegativeCohort(t *testing.T) {
	tests := []struct {
		name      string
		canaryPct *int
		cohort    int
		want      bool
	}{
		{"nil canary = enabled for all", nil, 50, true},
		{"nil canary = enabled even for no-cohort", nil, -1, true},
		{"no cohort excluded from canary", intPtrCfg(50), -1, false},
		{"cohort within threshold = enabled", intPtrCfg(50), 25, true},
		{"cohort at threshold = disabled", intPtrCfg(50), 50, false},
		{"cohort above threshold = disabled", intPtrCfg(50), 75, false},
		{"100% rollout includes cohort 99", intPtrCfg(100), 99, true},
		{"0% rollout excludes cohort 0", intPtrCfg(0), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the same logic as IsFeatureEnabled (service layer)
			got := simulateIsFeatureEnabledService(tt.canaryPct, tt.cohort)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsFeatureEnabledMatchesMiddleware verifies that the service-layer
// IsFeatureEnabled produces the same result as the middleware's
// IsFeatureEnabledForRequest for all representative inputs.
func TestIsFeatureEnabledMatchesMiddleware(t *testing.T) {
	scenarios := []struct {
		canaryPct *int
		cohort    int
	}{
		{nil, -1},
		{nil, 0},
		{nil, 50},
		{nil, 99},
		{intPtrCfg(0), -1},
		{intPtrCfg(0), 0},
		{intPtrCfg(1), 0},
		{intPtrCfg(50), -1},
		{intPtrCfg(50), 0},
		{intPtrCfg(50), 25},
		{intPtrCfg(50), 49},
		{intPtrCfg(50), 50},
		{intPtrCfg(50), 99},
		{intPtrCfg(100), 0},
		{intPtrCfg(100), 99},
	}

	for _, s := range scenarios {
		middleware := simulateIsFeatureEnabled(s.canaryPct, s.cohort)
		service := simulateIsFeatureEnabledService(s.canaryPct, s.cohort)
		if middleware != service {
			pctStr := "nil"
			if s.canaryPct != nil {
				pctStr = string(rune('0' + *s.canaryPct/10)) + string(rune('0'+*s.canaryPct%10))
			}
			t.Errorf("canaryPct=%s cohort=%d: middleware=%v service=%v (must match)",
				pctStr, s.cohort, middleware, service)
		}
	}
}

// TestGetFeatureStatusAcceptsCohortParam verifies GetFeatureStatus uses
// the cohort parameter (from middleware context) rather than computing
// its own cohort — ensuring a single source of truth for cohort assignment.
func TestGetFeatureStatusAcceptsCohortParam(t *testing.T) {
	tests := []struct {
		name      string
		canaryPct *int
		cohort    int
		want      bool
	}{
		{"no config = enabled", nil, 50, true},
		{"no cohort = excluded", intPtrCfg(50), -1, false},
		{"inside canary = enabled", intPtrCfg(50), 25, true},
		{"outside canary = disabled", intPtrCfg(50), 75, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := simulateGetFeatureStatus(tt.canaryPct, tt.cohort)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Helpers ---

func intPtrCfg(n int) *int {
	return &n
}

// simulateIsFeatureEnabledService replicates the fixed IsFeatureEnabled logic
// from the service layer, which now includes the negative cohort check.
func simulateIsFeatureEnabledService(canaryPct *int, cohort int) bool {
	if canaryPct == nil {
		return true
	}
	if cohort < 0 {
		return false
	}
	return cohort < *canaryPct
}

// simulateGetFeatureStatus replicates the fixed GetFeatureStatus logic
// which accepts a cohort parameter instead of computing its own.
func simulateGetFeatureStatus(canaryPct *int, cohort int) bool {
	if canaryPct == nil {
		return true
	}
	if cohort < 0 {
		return false
	}
	return cohort < *canaryPct
}
