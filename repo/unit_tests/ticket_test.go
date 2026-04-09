package unit_tests

import (
	"testing"
	"time"

	"campusrec/internal/repository"
)

func TestCalculateSLAResponseDeadline(t *testing.T) {
	// Monday 10:00 + 4 business hours = Monday 14:00
	start := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) // Monday
	deadline := repository.CalculateSLAResponseDeadline(start, 4)
	expected := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)

	if !deadline.Equal(expected) {
		t.Errorf("Deadline = %v, want %v", deadline, expected)
	}
}

func TestSLADeadlineSpansOvernight(t *testing.T) {
	// Monday 16:00 + 4 business hours = Tuesday 11:00 (2h Mon 16-18, 2h Tue 9-11)
	start := time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC) // Monday
	deadline := repository.CalculateSLAResponseDeadline(start, 4)
	expected := time.Date(2024, 1, 16, 11, 0, 0, 0, time.UTC) // Tuesday

	if !deadline.Equal(expected) {
		t.Errorf("Deadline = %v, want %v", deadline, expected)
	}
}

func TestSLADeadlineSkipsWeekend(t *testing.T) {
	// Friday 16:00 + 4 business hours = Monday 11:00 (2h Fri 16-18, skip Sat-Sun, 2h Mon 9-11)
	start := time.Date(2024, 1, 19, 16, 0, 0, 0, time.UTC) // Friday
	deadline := repository.CalculateSLAResponseDeadline(start, 4)
	expected := time.Date(2024, 1, 22, 11, 0, 0, 0, time.UTC) // Monday

	if !deadline.Equal(expected) {
		t.Errorf("Deadline = %v, want %v", deadline, expected)
	}
}

func TestSLADeadlineStartOnWeekend(t *testing.T) {
	// Saturday 12:00 + 4 business hours = Monday 13:00
	start := time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC) // Saturday
	deadline := repository.CalculateSLAResponseDeadline(start, 4)
	expected := time.Date(2024, 1, 22, 13, 0, 0, 0, time.UTC) // Monday

	if !deadline.Equal(expected) {
		t.Errorf("Deadline = %v, want %v", deadline, expected)
	}
}

func TestSLADeadlineStartBeforeBusinessHours(t *testing.T) {
	// Monday 7:00 + 4 business hours = Monday 13:00 (starts at 9:00)
	start := time.Date(2024, 1, 15, 7, 0, 0, 0, time.UTC) // Monday before 9
	deadline := repository.CalculateSLAResponseDeadline(start, 4)
	expected := time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC) // Monday

	if !deadline.Equal(expected) {
		t.Errorf("Deadline = %v, want %v", deadline, expected)
	}
}

func TestSLADeadlineStartAfterBusinessHours(t *testing.T) {
	// Monday 19:00 + 4 business hours = Tuesday 13:00
	start := time.Date(2024, 1, 15, 19, 0, 0, 0, time.UTC) // Monday after 18
	deadline := repository.CalculateSLAResponseDeadline(start, 4)
	expected := time.Date(2024, 1, 16, 13, 0, 0, 0, time.UTC) // Tuesday

	if !deadline.Equal(expected) {
		t.Errorf("Deadline = %v, want %v", deadline, expected)
	}
}
