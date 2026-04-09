package models

import "time"

type Registration struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	SessionID    string     `json:"session_id"`
	Status       string     `json:"status"`
	RegisteredAt *time.Time `json:"registered_at"`
	CanceledAt   *time.Time `json:"canceled_at"`
	CancelReason *string    `json:"cancel_reason"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Joined fields (populated on list queries)
	SessionTitle *string    `json:"session_title,omitempty"`
	Username     *string    `json:"username,omitempty"`
	DisplayName  *string    `json:"display_name,omitempty"`
}
