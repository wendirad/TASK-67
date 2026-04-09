package models

import "time"

type User struct {
	ID                  string     `json:"id"`
	Username            string     `json:"username"`
	PasswordHash        string     `json:"-"`
	Role                string     `json:"role"`
	DisplayName         string     `json:"display_name"`
	Email               *string    `json:"email"`
	Phone               *string    `json:"phone"`
	Status              string     `json:"status"`
	FailedLoginAttempts int        `json:"-"`
	LockedUntil         *time.Time `json:"-"`
	CanaryCohort        *int       `json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
