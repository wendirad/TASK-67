package services

import (
	"log"
	"strconv"
	"time"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

const defaultRegCloseMinutes = 120

type SessionService struct {
	sessionRepo  *repository.SessionRepository
	facilityRepo *repository.FacilityRepository
	configRepo   *repository.ConfigRepository
}

func NewSessionService(sessionRepo *repository.SessionRepository, facilityRepo *repository.FacilityRepository, configRepo *repository.ConfigRepository) *SessionService {
	return &SessionService{sessionRepo: sessionRepo, facilityRepo: facilityRepo, configRepo: configRepo}
}

func (s *SessionService) ListSessions(page, pageSize int, status, facility, search, fromDate, toDate string) ([]models.Session, int, error) {
	return s.sessionRepo.List(page, pageSize, status, facility, search, fromDate, toDate)
}

func (s *SessionService) GetSession(id string) (*models.Session, error) {
	return s.sessionRepo.FindByID(id)
}

type CreateSessionInput struct {
	Title                        string    `json:"title"`
	Description                  *string   `json:"description"`
	CoachName                    *string   `json:"coach_name"`
	FacilityID                   string    `json:"facility_id"`
	StartTime                    time.Time `json:"start_time"`
	EndTime                      time.Time `json:"end_time"`
	TotalSeats                   int       `json:"total_seats"`
	RegistrationCloseBeforeMin   *int      `json:"registration_close_before_minutes"`
}

func (s *SessionService) CreateSession(input CreateSessionInput, createdBy string) (*models.Session, int, string) {
	facility, err := s.facilityRepo.FindByID(input.FacilityID)
	if err != nil {
		log.Printf("Error finding facility %s: %v", input.FacilityID, err)
		return nil, 500, "Internal server error"
	}
	if facility == nil {
		return nil, 400, "Facility not found"
	}

	regCloseMin := s.resolveRegCloseDefault()
	if input.RegistrationCloseBeforeMin != nil {
		regCloseMin = *input.RegistrationCloseBeforeMin
	}

	session := &models.Session{
		Title:                      input.Title,
		Description:                input.Description,
		CoachName:                  input.CoachName,
		FacilityID:                 input.FacilityID,
		StartTime:                  input.StartTime,
		EndTime:                    input.EndTime,
		TotalSeats:                 input.TotalSeats,
		RegistrationCloseBeforeMin: regCloseMin,
		CreatedBy:                  createdBy,
	}

	if err := s.sessionRepo.Create(session); err != nil {
		log.Printf("Error creating session: %v", err)
		return nil, 500, "Internal server error"
	}

	session.FacilityName = facility.Name
	log.Printf("Session created: %s (%d seats)", session.Title, session.TotalSeats)
	return session, 201, ""
}

type UpdateSessionInput struct {
	Title                        *string    `json:"title"`
	Description                  *string    `json:"description"`
	CoachName                    *string    `json:"coach_name"`
	FacilityID                   *string    `json:"facility_id"`
	StartTime                    *time.Time `json:"start_time"`
	EndTime                      *time.Time `json:"end_time"`
	TotalSeats                   *int       `json:"total_seats"`
	RegistrationCloseBeforeMin   *int       `json:"registration_close_before_minutes"`
}

func (s *SessionService) UpdateSession(id string, input UpdateSessionInput) (*models.Session, int, string) {
	session, err := s.sessionRepo.FindByID(id)
	if err != nil {
		log.Printf("Error finding session %s: %v", id, err)
		return nil, 500, "Internal server error"
	}
	if session == nil {
		return nil, 404, "Session not found"
	}

	if session.Status == "in_progress" && (input.StartTime != nil || input.EndTime != nil) {
		return nil, 422, "Cannot change times of an in-progress session"
	}

	oldTotalSeats := session.TotalSeats

	if input.Title != nil {
		session.Title = *input.Title
	}
	if input.Description != nil {
		session.Description = input.Description
	}
	if input.CoachName != nil {
		session.CoachName = input.CoachName
	}
	if input.FacilityID != nil {
		facility, err := s.facilityRepo.FindByID(*input.FacilityID)
		if err != nil {
			log.Printf("Error finding facility %s: %v", *input.FacilityID, err)
			return nil, 500, "Internal server error"
		}
		if facility == nil {
			return nil, 400, "Facility not found"
		}
		session.FacilityID = *input.FacilityID
		session.FacilityName = facility.Name
	}
	if input.StartTime != nil {
		session.StartTime = *input.StartTime
	}
	if input.EndTime != nil {
		session.EndTime = *input.EndTime
	}
	if input.RegistrationCloseBeforeMin != nil {
		session.RegistrationCloseBeforeMin = *input.RegistrationCloseBeforeMin
	}

	if input.TotalSeats != nil {
		newTotal := *input.TotalSeats
		if newTotal < oldTotalSeats {
			consumingCount, err := s.sessionRepo.CountCapacityConsumingSeats(id)
			if err != nil {
				log.Printf("Error counting capacity seats for session %s: %v", id, err)
				return nil, 500, "Internal server error"
			}
			if newTotal < consumingCount {
				return nil, 422, "Cannot reduce total seats below the number of reserved/occupied/on_break seats"
			}
		}
		seatDiff := newTotal - oldTotalSeats
		session.TotalSeats = newTotal
		session.AvailableSeats = session.AvailableSeats + seatDiff
	}

	if !session.EndTime.After(session.StartTime) {
		return nil, 400, "End time must be after start time"
	}

	if session.TotalSeats != oldTotalSeats {
		if err := s.sessionRepo.UpdateWithSeats(session, oldTotalSeats); err != nil {
			log.Printf("Error updating session %s with seats: %v", id, err)
			return nil, 500, "Internal server error"
		}
	} else {
		if err := s.sessionRepo.Update(session); err != nil {
			log.Printf("Error updating session %s: %v", id, err)
			return nil, 500, "Internal server error"
		}
	}

	log.Printf("Session updated: %s", session.Title)
	return session, 200, ""
}

type UpdateSessionStatusInput struct {
	Status string `json:"status"`
}

func (s *SessionService) UpdateSessionStatus(id string, newStatus string) (*models.Session, int, string) {
	session, err := s.sessionRepo.FindByID(id)
	if err != nil {
		log.Printf("Error finding session %s: %v", id, err)
		return nil, 500, "Internal server error"
	}
	if session == nil {
		return nil, 404, "Session not found"
	}

	if newStatus == "canceled" {
		if err := s.sessionRepo.CancelSession(id); err != nil {
			log.Printf("Error canceling session %s: %v", id, err)
			return nil, 500, "Internal server error"
		}
		session.Status = "canceled"
		log.Printf("Session canceled: %s", session.Title)
		return session, 200, ""
	}

	if err := s.sessionRepo.UpdateStatus(id, newStatus); err != nil {
		log.Printf("Error updating session %s status: %v", id, err)
		return nil, 500, "Internal server error"
	}

	session.Status = newStatus
	log.Printf("Session %s status changed to %s", session.Title, newStatus)
	return session, 200, ""
}

// resolveRegCloseDefault returns the configured default registration closure
// minutes from config_entries, falling back to the hardcoded default.
func (s *SessionService) resolveRegCloseDefault() int {
	if s.configRepo == nil {
		return defaultRegCloseMinutes
	}
	entry, err := s.configRepo.FindByKey("session.reg_close_default_minutes")
	if err != nil || entry == nil {
		return defaultRegCloseMinutes
	}
	v, err := strconv.Atoi(entry.Value)
	if err != nil || v < 0 {
		return defaultRegCloseMinutes
	}
	return v
}
