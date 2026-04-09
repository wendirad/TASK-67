package unit_tests

import (
	"testing"

	"campusrec/internal/handlers"
)

func TestValidatePasswordTooShort(t *testing.T) {
	ok, msg := handlers.ValidatePassword("Short1!")
	if ok {
		t.Error("Expected validation to fail for short password")
	}
	if msg == "" {
		t.Error("Expected error message")
	}
}

func TestValidatePasswordNoUppercase(t *testing.T) {
	ok, _ := handlers.ValidatePassword("lowercaseonly1!")
	if ok {
		t.Error("Expected validation to fail without uppercase")
	}
}

func TestValidatePasswordNoLowercase(t *testing.T) {
	ok, _ := handlers.ValidatePassword("UPPERCASEONLY1!")
	if ok {
		t.Error("Expected validation to fail without lowercase")
	}
}

func TestValidatePasswordNoDigit(t *testing.T) {
	ok, _ := handlers.ValidatePassword("NoDigitsHere!!")
	if ok {
		t.Error("Expected validation to fail without digit")
	}
}

func TestValidatePasswordNoSpecial(t *testing.T) {
	ok, _ := handlers.ValidatePassword("NoSpecialChar1A")
	if ok {
		t.Error("Expected validation to fail without special character")
	}
}

func TestValidatePasswordValid(t *testing.T) {
	ok, msg := handlers.ValidatePassword("ValidPass123!!")
	if !ok {
		t.Errorf("Expected validation to pass, got error: %s", msg)
	}
}

func TestValidatePasswordMinLength(t *testing.T) {
	ok, _ := handlers.ValidatePassword("Abcdefghij1!") // exactly 12 chars
	if !ok {
		t.Error("Expected 12-char password to be valid")
	}
}

func TestValidatePhone(t *testing.T) {
	tests := []struct {
		phone string
		valid bool
	}{
		{"13800138000", true},
		{"15912345678", true},
		{"12345678901", false},
		{"1380013800", false},
		{"138001380001", false},
		{"abc", false},
		{"", false},
	}
	for _, tt := range tests {
		result := handlers.ValidatePhone(tt.phone)
		if result != tt.valid {
			t.Errorf("ValidatePhone(%q) = %v, want %v", tt.phone, result, tt.valid)
		}
	}
}

func TestValidatePostalCode(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"100000", true},
		{"518000", true},
		{"12345", false},
		{"1234567", false},
		{"abcdef", false},
		{"", false},
	}
	for _, tt := range tests {
		result := handlers.ValidatePostalCode(tt.code)
		if result != tt.valid {
			t.Errorf("ValidatePostalCode(%q) = %v, want %v", tt.code, result, tt.valid)
		}
	}
}
