package handlers

import (
	"regexp"
	"unicode"
)

// ValidatePassword checks the password meets security requirements:
// 12+ chars, at least one uppercase, one lowercase, one digit, one special character.
func ValidatePassword(password string) (bool, string) {
	if len(password) < 12 {
		return false, "Password must be at least 12 characters"
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return false, "Password must contain at least one uppercase letter"
	}
	if !hasLower {
		return false, "Password must contain at least one lowercase letter"
	}
	if !hasDigit {
		return false, "Password must contain at least one digit"
	}
	if !hasSpecial {
		return false, "Password must contain at least one special character"
	}

	return true, ""
}

var phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)
var postalCodeRegex = regexp.MustCompile(`^\d{6}$`)

// ValidatePhone checks if a phone number is valid (Chinese mobile format).
func ValidatePhone(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// ValidatePostalCode checks if a postal code is valid (6-digit Chinese format).
func ValidatePostalCode(code string) bool {
	return postalCodeRegex.MatchString(code)
}
