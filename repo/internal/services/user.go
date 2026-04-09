package services

import (
	"fmt"
	"log"

	"campusrec/internal/models"
	"campusrec/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

type CreateUserRequest struct {
	Username    string  `json:"username"`
	Password    string  `json:"password"`
	Role        string  `json:"role"`
	DisplayName string  `json:"display_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
}

// CreateUser creates a new user (admin operation).
// Returns (user, httpCode, errorMsg).
func (s *UserService) CreateUser(req CreateUserRequest) (*models.User, int, string) {
	exists, err := s.userRepo.UsernameExists(req.Username)
	if err != nil {
		log.Printf("Error checking username existence: %v", err)
		return nil, 500, "Internal server error"
	}
	if exists {
		return nil, 409, "Username already exists"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return nil, 500, "Internal server error"
	}

	user := &models.User{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
		Role:         req.Role,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Phone:        req.Phone,
		Status:       "active",
	}

	if err := s.userRepo.Create(user); err != nil {
		log.Printf("Error creating user: %v", err)
		return nil, 500, "Internal server error"
	}

	log.Printf("User created: %s (role=%s) by admin", user.Username, user.Role)
	return user, 201, ""
}

// UpdateUserStatus changes the status of a user (admin operation).
func (s *UserService) UpdateUserStatus(userID, status string) (*models.User, int, string) {
	user, err := s.userRepo.UpdateStatus(userID, status)
	if err != nil {
		log.Printf("Error updating user %s status: %v", userID, err)
		return nil, 500, "Internal server error"
	}
	if user == nil {
		return nil, 404, "User not found"
	}
	log.Printf("User %s status changed to %s", userID, status)
	return user, 200, ""
}

// ListUsers returns a paginated list of users (admin operation).
func (s *UserService) ListUsers(page, pageSize int, role, status, search string) ([]models.User, int, error) {
	users, total, err := s.userRepo.ListUsers(page, pageSize, role, status, search)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}
