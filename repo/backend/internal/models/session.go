package models

import "time"

type Session struct {
	ID                           string    `json:"id"`
	Title                        string    `json:"title"`
	Description                  *string   `json:"description"`
	CoachName                    *string   `json:"coach_name"`
	FacilityID                   string    `json:"facility_id"`
	FacilityName                 string    `json:"facility_name,omitempty"`
	StartTime                    time.Time `json:"start_time"`
	EndTime                      time.Time `json:"end_time"`
	TotalSeats                   int       `json:"total_seats"`
	AvailableSeats               int       `json:"available_seats"`
	RegistrationCloseBeforeMin   int       `json:"registration_close_before_minutes"`
	Status                       string    `json:"status"`
	CreatedBy                    string    `json:"created_by"`
	CreatedAt                    time.Time `json:"created_at"`
	UpdatedAt                    time.Time `json:"updated_at"`
	RegistrationOpen             bool      `json:"registration_open,omitempty"`
}
