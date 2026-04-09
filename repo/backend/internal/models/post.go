package models

import "time"

type Post struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Status        string    `json:"status"`
	ReportedCount int       `json:"reported_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Joined fields
	Username    *string `json:"username,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}

type PostReport struct {
	ID         string    `json:"id"`
	PostID     string    `json:"post_id"`
	ReportedBy string    `json:"reported_by"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

type ModerationDecision struct {
	ID          string    `json:"id"`
	PostID      string    `json:"post_id"`
	ModeratorID string    `json:"moderator_id"`
	Action      string    `json:"action"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}
