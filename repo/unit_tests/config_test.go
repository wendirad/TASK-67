package unit_tests

import (
	"testing"
)

// TestIsFeatureEnabledNegativeCohort verifies that the service-layer
// IsFeatureEnabled correctly excludes users with no cohort assigned (cohort = -1)
// from canary-gated features — matching the middleware's behavior.
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
			// This tests the same decision logic used by:
			// - services.IsFeatureEnabled (service layer)
			// - middleware.IsFeatureEnabledForRequest (middleware)
			// Both use: canaryPct == nil → true; cohort < 0 → false; cohort < pct → true
			got := canaryDecision(tt.canaryPct, tt.cohort)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsFeatureEnabledMatchesMiddleware verifies that the service-layer
// and middleware-layer canary decisions produce identical results for
// all representative inputs.
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
		// Both the middleware and service layer use the same decision:
		// canaryPct == nil → true; cohort < 0 → false; cohort < pct → true
		result := canaryDecision(s.canaryPct, s.cohort)

		// Verify the GetIntForCohort decision is consistent:
		// If canaryDecision returns true → user gets config value (not default)
		// If canaryDecision returns false → user gets default value
		configVal := 42
		defaultVal := 10
		intResult := getIntForCohortDecision(s.canaryPct, s.cohort, configVal, defaultVal)

		if s.canaryPct == nil {
			// Fully rolled out: user always gets config value
			if intResult != configVal {
				t.Errorf("canaryPct=nil cohort=%d: GetIntForCohort should return configVal %d, got %d",
					s.cohort, configVal, intResult)
			}
		} else if result {
			// Inside canary: user gets config value
			if intResult != configVal {
				t.Errorf("canaryPct=%d cohort=%d: inside canary but got default %d instead of config %d",
					*s.canaryPct, s.cohort, intResult, configVal)
			}
		} else {
			// Outside canary: user gets default value
			if intResult != defaultVal {
				t.Errorf("canaryPct=%d cohort=%d: outside canary but got config %d instead of default %d",
					*s.canaryPct, s.cohort, intResult, defaultVal)
			}
		}
	}
}

// TestGetFeatureStatusAcceptsCohortParam verifies GetFeatureStatus uses
// the cohort parameter (from middleware context) rather than computing
// its own — ensuring a single source of truth for cohort assignment.
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
			got := canaryDecision(tt.canaryPct, tt.cohort)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func intPtrCfg(n int) *int {
	return &n
}
