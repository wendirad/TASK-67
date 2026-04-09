package services

import (
	"fmt"
	"log"
	"time"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type RegistrationService struct {
	regRepo     *repository.RegistrationRepository
	sessionRepo *repository.SessionRepository
	userRepo    *repository.UserRepository
}

func NewRegistrationService(
	regRepo *repository.RegistrationRepository,
	sessionRepo *repository.SessionRepository,
	userRepo *repository.UserRepository,
) *RegistrationService {
	return &RegistrationService{
		regRepo:     regRepo,
		sessionRepo: sessionRepo,
		userRepo:    userRepo,
	}
}

// CreateRegistration creates a pending registration after validating session, user, and duplicates.
func (s *RegistrationService) CreateRegistration(userID, sessionID string) (*models.Registration, int, string) {
	// Validate user is active
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		log.Printf("Error finding user %s: %v", userID, err)
		return nil, 500, "Internal server error"
	}
	if user == nil || user.Status != "active" {
		return nil, 403, "User account is not active"
	}

	// Validate session exists and is open
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		log.Printf("Error finding session %s: %v", sessionID, err)
		return nil, 500, "Internal server error"
	}
	if session == nil {
		return nil, 404, "Session not found"
	}
	if session.Status != "open" {
		return nil, 422, "Session is not open for registration"
	}

	// Check registration closure
	closeTime := session.StartTime.Add(-time.Duration(session.RegistrationCloseBeforeMin) * time.Minute)
	if time.Now().After(closeTime) {
		return nil, 422, "Registration is closed for this session"
	}

	// Check for duplicate active registration
	exists, err := s.regRepo.HasActiveRegistration(userID, sessionID)
	if err != nil {
		log.Printf("Error checking existing registration: %v", err)
		return nil, 500, "Internal server error"
	}
	if exists {
		return nil, 409, "Already registered for this session"
	}

	reg := &models.Registration{
		UserID:    userID,
		SessionID: sessionID,
	}
	if err := s.regRepo.Create(reg); err != nil {
		log.Printf("Error creating registration: %v", err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Registration created: user=%s session=%s status=pending", userID, sessionID)
	return reg, 201, ""
}

// ApproveRegistration transitions pending → approved.
func (s *RegistrationService) ApproveRegistration(regID string) (*models.Registration, int, string) {
	reg, err := s.regRepo.FindByID(regID)
	if err != nil {
		log.Printf("Error finding registration %s: %v", regID, err)
		return nil, 500, "Internal server error"
	}
	if reg == nil {
		return nil, 404, "Registration not found"
	}
	if reg.Status != "pending" {
		return nil, 422, fmt.Sprintf("Cannot approve registration in '%s' state", reg.Status)
	}

	if err := s.regRepo.Approve(regID); err != nil {
		log.Printf("Error approving registration %s: %v", regID, err)
		return nil, 500, "Internal server error"
	}

	reg.Status = "approved"
	log.Printf("Registration approved: %s", regID)
	return reg, 200, ""
}

// RejectRegistration transitions pending → rejected.
func (s *RegistrationService) RejectRegistration(regID, reason string) (*models.Registration, int, string) {
	reg, err := s.regRepo.FindByID(regID)
	if err != nil {
		log.Printf("Error finding registration %s: %v", regID, err)
		return nil, 500, "Internal server error"
	}
	if reg == nil {
		return nil, 404, "Registration not found"
	}
	if reg.Status != "pending" {
		return nil, 422, fmt.Sprintf("Cannot reject registration in '%s' state", reg.Status)
	}

	if err := s.regRepo.Reject(regID, reason); err != nil {
		log.Printf("Error rejecting registration %s: %v", regID, err)
		return nil, 500, "Internal server error"
	}

	reg.Status = "rejected"
	reg.CancelReason = &reason
	log.Printf("Registration rejected: %s", regID)
	return reg, 200, ""
}

// ConfirmRegistration transitions approved → registered (with seat) or → waitlisted (if full).
func (s *RegistrationService) ConfirmRegistration(regID, userID string) (*models.Registration, int, string) {
	reg, err := s.regRepo.FindByID(regID)
	if err != nil {
		log.Printf("Error finding registration %s: %v", regID, err)
		return nil, 500, "Internal server error"
	}
	if reg == nil {
		return nil, 404, "Registration not found"
	}
	if reg.UserID != userID {
		return nil, 403, "Not your registration"
	}
	if reg.Status != "approved" {
		return nil, 422, fmt.Sprintf("Cannot confirm registration in '%s' state", reg.Status)
	}

	newStatus, err := s.regRepo.ConfirmRegistration(regID, reg.SessionID, userID)
	if err != nil {
		log.Printf("Error confirming registration %s: %v", regID, err)
		return nil, 500, "Internal server error"
	}

	reg.Status = newStatus
	if newStatus == "waitlisted" {
		log.Printf("Registration waitlisted: %s (no seats available)", regID)
		return reg, 200, "No seats available. You have been added to the waitlist."
	}

	log.Printf("Registration confirmed: %s (seat reserved)", regID)
	return reg, 200, ""
}

// CancelRegistration cancels from any active state with appropriate side effects.
func (s *RegistrationService) CancelRegistration(regID, userID string, isAdmin bool) (*models.Registration, int, string) {
	reg, err := s.regRepo.FindByID(regID)
	if err != nil {
		log.Printf("Error finding registration %s: %v", regID, err)
		return nil, 500, "Internal server error"
	}
	if reg == nil {
		return nil, 404, "Registration not found"
	}
	if !isAdmin && reg.UserID != userID {
		return nil, 403, "Not your registration"
	}

	cancelableStatuses := map[string]bool{
		"pending": true, "approved": true, "registered": true, "waitlisted": true,
	}
	if !cancelableStatuses[reg.Status] {
		return nil, 422, fmt.Sprintf("Cannot cancel registration in '%s' state", reg.Status)
	}

	if err := s.regRepo.CancelRegistration(regID, reg.SessionID, reg.UserID, reg.Status); err != nil {
		log.Printf("Error canceling registration %s: %v", regID, err)
		return nil, 500, "Internal server error"
	}

	reg.Status = "canceled"
	log.Printf("Registration canceled: %s (was %s)", regID, reg.Status)
	return reg, 200, ""
}

// ListUserRegistrations returns paginated registrations for a member.
func (s *RegistrationService) ListUserRegistrations(userID string, page, pageSize int, status string) ([]models.Registration, int, error) {
	return s.regRepo.ListByUser(userID, page, pageSize, status)
}

// ListAllRegistrations returns paginated registrations for admin.
func (s *RegistrationService) ListAllRegistrations(page, pageSize int, sessionID, userID, status string) ([]models.Registration, int, error) {
	return s.regRepo.ListAll(page, pageSize, sessionID, userID, status)
}
