package handlers

import (
	"regexp"

	"campusrec/internal/models"
)

// ValidatePassword delegates to models.ValidatePassword so the same rules
// are enforced everywhere (admin user creation, password change, import).
func ValidatePassword(password string) (bool, string) {
	return models.ValidatePassword(password)
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
