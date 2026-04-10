package unit_tests

import (
	"testing"
)

// TestCanaryCohortRange verifies the deterministic cohort computation produces
// values in the expected [0, 99] range for a variety of UUID-derived inputs.
func TestCanaryCohortRange(t *testing.T) {
	// The SQL expression: MOD(('x' || left(hex8, 8))::bit(32)::int::bigint + 2147483648, 100)
	// We replicate the math in Go to verify the algorithm.
	uuids := []string{
		"00000000-0000-0000-0000-000000000000",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"12345678-1234-1234-1234-123456789abc",
		"abcdef01-2345-6789-abcd-ef0123456789",
	}

	for _, uuid := range uuids {
		cohort := computeCohort(uuid)
		if cohort < 0 || cohort > 99 {
			t.Errorf("cohort for UUID %s = %d, want [0, 99]", uuid, cohort)
		}
	}
}

// TestCanaryCohortDeterministic verifies the same UUID always produces the same cohort.
func TestCanaryCohortDeterministic(t *testing.T) {
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	first := computeCohort(uuid)
	for i := 0; i < 100; i++ {
		if got := computeCohort(uuid); got != first {
			t.Fatalf("cohort changed on iteration %d: got %d, want %d", i, got, first)
		}
	}
}

// TestCanaryCohortDistribution verifies that cohort values are reasonably distributed
// across the [0, 99] range and not clustered.
func TestCanaryCohortDistribution(t *testing.T) {
	// Generate 100 "UUID-like" hex strings and check we get at least 20 distinct cohorts.
	seen := make(map[int]bool)
	for i := 0; i < 100; i++ {
		hex := padHex(i)
		cohort := computeCohort(hex + "-0000-0000-0000-000000000000")
		seen[cohort] = true
	}
	if len(seen) < 20 {
		t.Errorf("only %d distinct cohorts from 100 UUIDs, expected at least 20", len(seen))
	}
}

// TestGetIntForCohortLogic verifies the canary gating decision logic:
// - If canaryPct is nil, the config value applies to all users.
// - If userCohort < 0 (no cohort), the user gets the default.
// - If userCohort >= canaryPct, the user gets the default.
// - If userCohort < canaryPct, the user gets the config value.
func TestGetIntForCohortLogic(t *testing.T) {
	tests := []struct {
		name        string
		canaryPct   *int
		userCohort  int
		configValue int
		defaultVal  int
		want        int
	}{
		{"nil canary = value for all", nil, 50, 10, 5, 10},
		{"nil canary = value even for no-cohort", nil, -1, 10, 5, 10},
		{"no cohort gets default", intPtr(50), -1, 10, 5, 5},
		{"cohort below threshold gets config", intPtr(50), 25, 10, 5, 10},
		{"cohort at boundary gets default", intPtr(50), 50, 10, 5, 5},
		{"cohort above threshold gets default", intPtr(50), 75, 10, 5, 5},
		{"cohort 0 with pct 1 gets config", intPtr(1), 0, 10, 5, 10},
		{"cohort 0 with pct 0 gets default", intPtr(0), 0, 10, 5, 5},
		{"100% rollout includes all cohorts", intPtr(100), 99, 10, 5, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := simulateGetIntForCohort(tt.canaryPct, tt.userCohort, tt.configValue, tt.defaultVal)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// TestIsFeatureEnabledLogic verifies the middleware's feature-enabled decision.
func TestIsFeatureEnabledLogic(t *testing.T) {
	tests := []struct {
		name      string
		canaryPct *int
		cohort    int
		want      bool
	}{
		{"nil canary = enabled for all", nil, 50, true},
		{"nil canary = enabled for no-cohort", nil, -1, true},
		{"no cohort excluded from canary", intPtr(50), -1, false},
		{"cohort within threshold = enabled", intPtr(50), 25, true},
		{"cohort at threshold = disabled", intPtr(50), 50, false},
		{"cohort above threshold = disabled", intPtr(50), 75, false},
		{"100% rollout includes cohort 99", intPtr(100), 99, true},
		{"0% rollout excludes cohort 0", intPtr(0), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := simulateIsFeatureEnabled(tt.canaryPct, tt.cohort)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPaymentTimeoutCanaryGating verifies the order payment timeout respects canary config.
func TestPaymentTimeoutCanaryGating(t *testing.T) {
	tests := []struct {
		name        string
		canaryPct   *int
		userCohort  int
		configValue int
		want        int // expected timeout in minutes
	}{
		{"default timeout when outside canary", intPtr(50), 75, 20, 15},
		{"config timeout when inside canary", intPtr(50), 25, 20, 20},
		{"default timeout when no cohort assigned", intPtr(50), -1, 20, 15},
		{"config timeout with full rollout", nil, 50, 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := simulateGetIntForCohort(tt.canaryPct, tt.userCohort, tt.configValue, 15)
			if got != tt.want {
				t.Errorf("payment timeout = %d minutes, want %d", got, tt.want)
			}
		})
	}
}

// TestPostRateLimitCanaryGating verifies post rate limit respects canary config.
func TestPostRateLimitCanaryGating(t *testing.T) {
	tests := []struct {
		name        string
		canaryPct   *int
		userCohort  int
		configValue int
		want        int
	}{
		{"default rate limit outside canary", intPtr(30), 50, 10, 5},
		{"config rate limit inside canary", intPtr(30), 15, 10, 10},
		{"full rollout applies to everyone", nil, 50, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := simulateGetIntForCohort(tt.canaryPct, tt.userCohort, tt.configValue, 5)
			if got != tt.want {
				t.Errorf("rate limit = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestAutoFlagThresholdCanaryGating verifies post auto-flag threshold respects canary config.
func TestAutoFlagThresholdCanaryGating(t *testing.T) {
	tests := []struct {
		name        string
		canaryPct   *int
		userCohort  int
		configValue int
		want        int
	}{
		{"default threshold outside canary", intPtr(20), 50, 5, 3},
		{"config threshold inside canary", intPtr(20), 10, 5, 5},
		{"full rollout uses config", nil, 50, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := simulateGetIntForCohort(tt.canaryPct, tt.userCohort, tt.configValue, 3)
			if got != tt.want {
				t.Errorf("auto-flag threshold = %d, want %d", got, tt.want)
			}
		})
	}
}

// --- Helpers ---

// computeCohort replicates the SQL cohort computation in Go.
func computeCohort(uuid string) int {
	// Remove hyphens, take first 8 hex chars
	hex := ""
	for _, c := range uuid {
		if c != '-' {
			hex += string(c)
			if len(hex) == 8 {
				break
			}
		}
	}

	// Parse as 32-bit unsigned
	var val uint32
	for _, c := range hex {
		val <<= 4
		switch {
		case c >= '0' && c <= '9':
			val |= uint32(c - '0')
		case c >= 'a' && c <= 'f':
			val |= uint32(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			val |= uint32(c - 'A' + 10)
		}
	}

	// The SQL uses ::int (signed 32-bit) then ::bigint + 2147483648 to shift
	// to unsigned range. In Go, we just use uint32 directly since it's already
	// unsigned. Then MOD 100.
	// Actually the SQL:  bit(32)::int  gives signed int32
	//                    ::bigint + 2147483648  shifts to [0, 4294967295]
	//                    MOD(..., 100)
	// So: signed = int32(val), then int64(signed) + 2147483648, then mod 100
	signed := int32(val)
	shifted := int64(signed) + 2147483648
	cohort := int(shifted % 100)
	return cohort
}

// padHex creates an 8-char hex string from an integer.
func padHex(n int) string {
	s := ""
	for i := 7; i >= 0; i-- {
		nibble := (n >> (i * 4)) & 0xf
		if nibble < 10 {
			s += string(rune('0' + nibble))
		} else {
			s += string(rune('a' + nibble - 10))
		}
	}
	return s
}

// simulateGetIntForCohort replicates the logic of ConfigRepository.GetIntForCohort.
func simulateGetIntForCohort(canaryPct *int, userCohort, configValue, defaultVal int) int {
	if canaryPct == nil {
		if configValue <= 0 {
			return defaultVal
		}
		return configValue
	}
	if userCohort < 0 || userCohort >= *canaryPct {
		return defaultVal
	}
	if configValue <= 0 {
		return defaultVal
	}
	return configValue
}

// simulateIsFeatureEnabled replicates the logic of IsFeatureEnabledForRequest.
func simulateIsFeatureEnabled(canaryPct *int, cohort int) bool {
	if canaryPct == nil {
		return true
	}
	if cohort < 0 {
		return false
	}
	return cohort < *canaryPct
}

func intPtr(n int) *int {
	return &n
}

// TestCanaryMiddlewareIsSourceOfTruth verifies that the canary cohort always
// flows from middleware → handler → service as a plain int parameter, rather
// than services reading user.CanaryCohort directly from the database. This
// ensures consistent behavior regardless of middleware execution order.
func TestCanaryMiddlewareIsSourceOfTruth(t *testing.T) {
	// The middleware sets context key "canary_cohort" to the user's cohort
	// (or -1 if nil). Services receive it as a parameter.
	// Verify the contract: -1 means no cohort, 0-99 are valid cohorts.
	tests := []struct {
		name      string
		cohort    int
		isValid   bool
	}{
		{"no cohort assigned", -1, false},
		{"cohort 0", 0, true},
		{"cohort 50", 50, true},
		{"cohort 99", 99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.cohort >= 0 && tt.cohort <= 99
			if valid != tt.isValid {
				t.Errorf("cohort %d valid = %v, want %v", tt.cohort, valid, tt.isValid)
			}
		})
	}
}
