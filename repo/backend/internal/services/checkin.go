package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"time"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type CheckInService struct {
	checkinRepo  *repository.CheckInRepository
	regRepo      *repository.RegistrationRepository
	sessionRepo  *repository.SessionRepository
	facilityRepo *repository.FacilityRepository
	jwtSecret    string
}

func NewCheckInService(
	checkinRepo *repository.CheckInRepository,
	regRepo *repository.RegistrationRepository,
	sessionRepo *repository.SessionRepository,
	facilityRepo *repository.FacilityRepository,
	jwtSecret string,
) *CheckInService {
	return &CheckInService{
		checkinRepo:  checkinRepo,
		regRepo:      regRepo,
		sessionRepo:  sessionRepo,
		facilityRepo: facilityRepo,
		jwtSecret:    jwtSecret,
	}
}

// PerformCheckIn validates and creates a check-in from a staff kiosk request.
func (s *CheckInService) PerformCheckIn(staffUserID string, req *models.CheckInRequest) (*models.CheckIn, int, string) {
	// Validate registration
	reg, err := s.regRepo.FindByID(req.RegistrationID)
	if err != nil {
		log.Printf("Error finding registration %s: %v", req.RegistrationID, err)
		return nil, 500, "Internal server error"
	}
	if reg == nil {
		return nil, 404, "Registration not found"
	}
	if reg.Status != "registered" {
		return nil, 422, fmt.Sprintf("Registration is in '%s' state, must be 'registered' to check in", reg.Status)
	}

	// Check if already checked in
	existing, err := s.checkinRepo.FindByRegistration(req.RegistrationID)
	if err != nil {
		log.Printf("Error checking existing check-in: %v", err)
		return nil, 500, "Internal server error"
	}
	if existing != nil {
		return nil, 409, "Already checked in"
	}

	// Validate session timing
	session, err := s.sessionRepo.FindByID(reg.SessionID)
	if err != nil {
		log.Printf("Error finding session %s: %v", reg.SessionID, err)
		return nil, 500, "Internal server error"
	}
	if session == nil {
		return nil, 404, "Session not found"
	}

	now := time.Now()
	windowStart := session.StartTime.Add(-5 * time.Minute)
	noShowCutoff := session.StartTime.Add(10 * time.Minute)

	if now.Before(windowStart) {
		return nil, 422, "Check-in window not yet open (opens 5 minutes before session start)"
	}
	if now.After(noShowCutoff) {
		return nil, 422, "Check-in window has closed (10 minutes past start)"
	}
	if session.Status != "in_progress" && session.Status != "open" {
		return nil, 422, fmt.Sprintf("Session is '%s', check-in requires 'open' or 'in_progress'", session.Status)
	}

	// Validate kiosk device token
	facility, err := s.facilityRepo.FindByID(session.FacilityID)
	if err != nil {
		log.Printf("Error finding facility %s: %v", session.FacilityID, err)
		return nil, 500, "Internal server error"
	}
	if facility == nil {
		return nil, 500, "Facility not found for session"
	}
	if facility.KioskDeviceToken == nil || *facility.KioskDeviceToken != req.KioskDeviceToken {
		log.Printf("Unauthorized kiosk device attempt for facility %s", facility.ID)
		return nil, 403, "Unauthorized kiosk device"
	}

	// Enforce facility check-in mode
	method := "qr_scan"
	if facility.CheckinMode == "staff_qr_bluetooth" {
		if !req.BluetoothConfirmed {
			return nil, 422, "Bluetooth beacon confirmation required for this facility"
		}
		if facility.BluetoothBeaconID == nil || req.BluetoothBeaconID != *facility.BluetoothBeaconID {
			return nil, 422, "Bluetooth beacon ID does not match facility configuration"
		}
		method = "qr_scan_bluetooth"
	}

	// Perform the check-in (atomic: create record + seat reserved→occupied)
	ci, err := s.checkinRepo.PerformCheckIn(req.RegistrationID, reg.UserID, reg.SessionID, staffUserID, method)
	if err != nil {
		log.Printf("Error performing check-in: %v", err)
		return nil, 422, err.Error()
	}

	log.Printf("Check-in performed: registration=%s user=%s session=%s seat=%s method=%s staff=%s",
		req.RegistrationID, reg.UserID, reg.SessionID, ci.SeatID, method, staffUserID)
	return ci, 201, ""
}

// GetCheckIn returns a check-in by ID with ownership check.
func (s *CheckInService) GetCheckIn(checkInID, userID, role string) (*models.CheckIn, int, string) {
	ci, err := s.checkinRepo.FindByID(checkInID)
	if err != nil {
		log.Printf("Error finding check-in %s: %v", checkInID, err)
		return nil, 500, "Internal server error"
	}
	if ci == nil {
		return nil, 404, "Check-in not found"
	}
	if role != "admin" && role != "staff" && ci.UserID != userID {
		return nil, 403, "Not your check-in"
	}
	return ci, 200, ""
}

// StartBreak initiates a temporary leave.
func (s *CheckInService) StartBreak(checkInID, userID, role string) (int, string) {
	ci, err := s.checkinRepo.FindByID(checkInID)
	if err != nil {
		log.Printf("Error finding check-in %s: %v", checkInID, err)
		return 500, "Internal server error"
	}
	if ci == nil {
		return 404, "Check-in not found"
	}
	if role != "admin" && role != "staff" && ci.UserID != userID {
		return 403, "Not your check-in"
	}
	if ci.Status != "active" {
		return 422, fmt.Sprintf("Check-in is '%s', must be 'active' to start a break", ci.Status)
	}

	// Check break count limit
	maxBreaks, _ := s.checkinRepo.GetBreakMaxCount()
	if ci.BreakCount >= maxBreaks {
		return 422, fmt.Sprintf("Maximum break limit reached (%d)", maxBreaks)
	}

	if err := s.checkinRepo.StartBreak(checkInID, ci.SeatID); err != nil {
		log.Printf("Error starting break: %v", err)
		return 500, "Internal server error"
	}

	log.Printf("Break started: checkin=%s user=%s", checkInID, ci.UserID)
	return 200, ""
}

// ReturnFromBreak ends a break. If overrun, releases the seat.
func (s *CheckInService) ReturnFromBreak(checkInID, userID, role string) (int, string) {
	ci, err := s.checkinRepo.FindByID(checkInID)
	if err != nil {
		log.Printf("Error finding check-in %s: %v", checkInID, err)
		return 500, "Internal server error"
	}
	if ci == nil {
		return 404, "Check-in not found"
	}
	if role != "admin" && role != "staff" && ci.UserID != userID {
		return 403, "Not your check-in"
	}
	if ci.Status != "on_break" {
		return 422, fmt.Sprintf("Check-in is '%s', must be 'on_break' to return", ci.Status)
	}

	if ci.LastBreakStart == nil {
		return 500, "Break start time not recorded"
	}

	breakMinutes := int(math.Ceil(time.Since(*ci.LastBreakStart).Minutes()))
	maxMinutes, _ := s.checkinRepo.GetBreakMaxMinutes()

	if breakMinutes > maxMinutes {
		// Break overrun — release seat
		if err := s.checkinRepo.ReleaseSeatFromBreak(checkInID, ci.SeatID, ci.SessionID, breakMinutes); err != nil {
			log.Printf("Error releasing seat from break overrun: %v", err)
			return 500, "Internal server error"
		}
		log.Printf("Break overrun: checkin=%s user=%s duration=%dmin (max %d), seat released",
			checkInID, ci.UserID, breakMinutes, maxMinutes)
		return 422, fmt.Sprintf("Break exceeded %d minute limit. Your seat has been released.", maxMinutes)
	}

	if err := s.checkinRepo.ReturnFromBreak(checkInID, ci.SeatID, breakMinutes); err != nil {
		log.Printf("Error returning from break: %v", err)
		return 500, "Internal server error"
	}

	log.Printf("Break ended: checkin=%s user=%s duration=%dmin", checkInID, ci.UserID, breakMinutes)
	return 200, ""
}

// GenerateSessionQR generates a signed QR code payload for a session.
func (s *CheckInService) GenerateSessionQR(sessionID string) (*models.QRCodeResponse, int, string) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		log.Printf("Error finding session %s: %v", sessionID, err)
		return nil, 500, "Internal server error"
	}
	if session == nil {
		return nil, 404, "Session not found"
	}

	timestamp := time.Now().Unix()
	payload := fmt.Sprintf("CAMPUSREC:CHECKIN:%s:%d", sessionID, timestamp)

	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	qrContent := fmt.Sprintf("%s:%s", payload, signature)
	validUntil := time.Now().Add(10 * time.Minute)

	return &models.QRCodeResponse{
		QRContent:  qrContent,
		ValidUntil: validUntil,
	}, 200, ""
}
