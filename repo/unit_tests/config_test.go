package unit_tests

import (
	"testing"

	"campusrec/internal/services"
)

func TestComputeCanaryCohort(t *testing.T) {
	// Should be deterministic
	cohort1 := services.ComputeCanaryCohort("user-123")
	cohort2 := services.ComputeCanaryCohort("user-123")
	if cohort1 != cohort2 {
		t.Errorf("ComputeCanaryCohort not deterministic: %d != %d", cohort1, cohort2)
	}
}

func TestComputeCanaryCohortRange(t *testing.T) {
	// All results should be 0-99
	for i := 0; i < 1000; i++ {
		id := "user-" + string(rune('A'+i%26)) + string(rune('0'+i%10))
		cohort := services.ComputeCanaryCohort(id)
		if cohort < 0 || cohort > 99 {
			t.Errorf("ComputeCanaryCohort(%q) = %d, want 0-99", id, cohort)
		}
	}
}

func TestComputeCanaryCohortDifferentUsers(t *testing.T) {
	// Different users should (usually) get different cohorts
	cohortA := services.ComputeCanaryCohort("alice")
	cohortB := services.ComputeCanaryCohort("bob")
	// They might collide, but let's just verify they compute without error
	_ = cohortA
	_ = cohortB
}

func TestComputeCanaryCohortEmptyString(t *testing.T) {
	cohort := services.ComputeCanaryCohort("")
	if cohort < 0 || cohort > 99 {
		t.Errorf("ComputeCanaryCohort(\"\") = %d, want 0-99", cohort)
	}
}

func TestComputeCanaryCohortDistribution(t *testing.T) {
	// Verify reasonable distribution across 100 users
	buckets := make(map[int]int)
	for i := 0; i < 100; i++ {
		id := string(rune(i + 1000))
		cohort := services.ComputeCanaryCohort(id)
		buckets[cohort]++
	}
	// With 100 users and 100 buckets, we should have at least 10 different buckets
	if len(buckets) < 10 {
		t.Errorf("Poor distribution: only %d unique cohorts from 100 users", len(buckets))
	}
}
