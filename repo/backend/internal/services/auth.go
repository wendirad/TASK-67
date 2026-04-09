package services

import (
	"fmt"
	"log"
	"math"
	"time"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
	"campusrec/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo  *repository.UserRepository
	jwtSecret string
}

func NewAuthService(userRepo *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{userRepo: userRepo, jwtSecret: jwtSecret}
}

type LoginResult struct {
	Token string
	User  *models.User
}

// Login authenticates a user and returns a JWT token.
// Returns (result, httpCode, errorMsg).
func (s *AuthService) Login(username, password string) (*LoginResult, int, string) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		log.Printf("Error finding user %q: %v", username, err)
		return nil, 500, "Internal server error"
	}
	if user == nil {
		return nil, 401, "Invalid username or password"
	}

	if user.Status != "active" {
		return nil, 401, "Invalid username or password"
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		remaining := math.Ceil(time.Until(*user.LockedUntil).Minutes())
		return nil, 423, fmt.Sprintf("Account locked. Try again in %.0f minutes", remaining)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		if incrErr := s.userRepo.IncrementFailedAttempts(user.ID); incrErr != nil {
			log.Printf("Error incrementing failed attempts for user %s: %v", user.ID, incrErr)
		}
		log.Printf("Failed login attempt for user %q", username)
		return nil, 401, "Invalid username or password"
	}

	if err := s.userRepo.ResetFailedAttempts(user.ID); err != nil {
		log.Printf("Error resetting failed attempts for user %s: %v", user.ID, err)
	}

	token, err := middleware.GenerateToken(user.ID, user.Role, user.Username, user.DisplayName, s.jwtSecret)
	if err != nil {
		log.Printf("Error generating token for user %s: %v", user.ID, err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Successful login for user %q (role=%s)", username, user.Role)

	return &LoginResult{Token: token, User: user}, 200, ""
}

// ChangePassword verifies the current password and updates to the new one.
// Returns (httpCode, errorMsg).
func (s *AuthService) ChangePassword(userID, currentPassword, newPassword string) (int, string) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		log.Printf("Error finding user %s for password change: %v", userID, err)
		return 500, "Internal server error"
	}
	if user == nil {
		return 404, "User not found"
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return 400, "Current password is incorrect"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing new password for user %s: %v", userID, err)
		return 500, "Internal server error"
	}

	if err := s.userRepo.UpdatePasswordHash(userID, string(hashedPassword)); err != nil {
		log.Printf("Error updating password for user %s: %v", userID, err)
		return 500, "Internal server error"
	}

	log.Printf("Password changed for user %s", userID)
	return 200, ""
}

// GetUserProfile returns the user profile for the given user ID.
func (s *AuthService) GetUserProfile(userID string) (*models.User, error) {
	return s.userRepo.FindByID(userID)
}
