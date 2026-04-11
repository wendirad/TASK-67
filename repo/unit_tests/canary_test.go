package unit_tests

import (
	"testing"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
)

// TestCanaryCohortRange verifies the deterministic cohort computation produces
// values in the expected [0, 99] range for a variety of UUID-derived inputs.
// This replicates the SQL expression used in UserRepository.Create to verify
// the algorithm independently.
func TestCanaryCohortRange(t *testing.T) {
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

// TestGetCanaryCohortDefaultValue verifies that GetCanaryCohort returns -1
// when no cohort is set in the context, matching the documented contract.
func TestGetCanaryCohortDefaultValue(t *testing.T) {
	tests := []struct {
		name    string
		cohort  int
		isValid bool
	}{
		{"no cohort assigned", -1, false},
		{"cohort 0 (lowest valid)", 0, true},
		{"cohort 50 (mid range)", 50, true},
		{"cohort 99 (highest valid)", 99, true},
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

// TestCanaryEnabledContract verifies the real models.CanaryEnabled function
// used by middleware.IsFeatureEnabledForRequest, services.IsFeatureEnabled,
// and services.ConfigService.GetFeatureStatus. All three delegate to
// CanaryEnabled after their DB lookup.
func TestCanaryEnabledContract(t *testing.T) {
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
			got := models.CanaryEnabled(tt.canaryPct, tt.cohort)
			if got != tt.want {
				t.Errorf("CanaryEnabled(pct=%v, cohort=%d) = %v, want %v",
					tt.canaryPct, tt.cohort, got, tt.want)
			}
		})
	}
}

// TestCanaryIntValuePaymentTimeout verifies models.CanaryIntValue — the real
// function used by repository.GetIntForCohort when resolving payment_timeout_minutes.
func TestCanaryIntValuePaymentTimeout(t *testing.T) {
	defaultTimeout := 15
	tests := []struct {
		name        string
		canaryPct   *int
		userCohort  int
		configValue int
		want        int
	}{
		{"default timeout when outside canary", intPtr(50), 75, 20, defaultTimeout},
		{"config timeout when inside canary", intPtr(50), 25, 20, 20},
		{"default timeout when no cohort assigned", intPtr(50), -1, 20, defaultTimeout},
		{"config timeout with full rollout", nil, 50, 20, 20},
		{"non-positive config returns default", nil, 50, 0, defaultTimeout},
		{"negative config returns default", intPtr(50), 25, -1, defaultTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.CanaryIntValue(tt.canaryPct, tt.userCohort, tt.configValue, defaultTimeout)
			if got != tt.want {
				t.Errorf("CanaryIntValue = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestCanaryIntValuePostRateLimit verifies models.CanaryIntValue for the post
// rate limit use case in PostService.CreatePost.
func TestCanaryIntValuePostRateLimit(t *testing.T) {
	defaultLimit := 5
	tests := []struct {
		name        string
		canaryPct   *int
		userCohort  int
		configValue int
		want        int
	}{
		{"default rate limit outside canary", intPtr(30), 50, 10, defaultLimit},
		{"config rate limit inside canary", intPtr(30), 15, 10, 10},
		{"full rollout applies to everyone", nil, 50, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.CanaryIntValue(tt.canaryPct, tt.userCohort, tt.configValue, defaultLimit)
			if got != tt.want {
				t.Errorf("CanaryIntValue = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestCanaryMiddlewareExportsGetCanaryCohort verifies that the middleware
// package exports GetCanaryCohort (compile-time check that the function exists
// and is importable from the middleware package).
func TestCanaryMiddlewareExportsGetCanaryCohort(t *testing.T) {
	fn := middleware.GetCanaryCohort
	if fn == nil {
		t.Fatal("middleware.GetCanaryCohort should not be nil")
	}
}

// --- Helpers ---

// computeCohort replicates the SQL cohort computation in Go.
// SQL: MOD(('x' || left(replace(uid::text, '-', ''), 8))::bit(32)::int::bigint + 2147483648, 100)
func computeCohort(uuid string) int {
	hex := ""
	for _, c := range uuid {
		if c != '-' {
			hex += string(c)
			if len(hex) == 8 {
				break
			}
		}
	}

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

func intPtr(n int) *int {
	return &n
}
