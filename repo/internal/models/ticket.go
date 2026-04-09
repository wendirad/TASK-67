package models

import "time"

type Ticket struct {
	ID                   string     `json:"id"`
	TicketNumber         string     `json:"ticket_number"`
	Type                 string     `json:"type"`
	Subject              string     `json:"subject"`
	Description          string     `json:"description"`
	Status               string     `json:"status"`
	Priority             string     `json:"priority"`
	AssignedTo           *string    `json:"assigned_to"`
	CreatedBy            string     `json:"created_by"`
	RelatedEntityType    *string    `json:"related_entity_type,omitempty"`
	RelatedEntityID      *string    `json:"related_entity_id,omitempty"`
	SLAResponseDeadline  *time.Time `json:"sla_response_deadline,omitempty"`
	SLAResolutionDeadline *time.Time `json:"sla_resolution_deadline,omitempty"`
	SLAResponseMet       *bool      `json:"sla_response_met,omitempty"`
	SLAResolutionMet     *bool      `json:"sla_resolution_met,omitempty"`
	RespondedAt          *time.Time `json:"responded_at,omitempty"`
	ResolvedAt           *time.Time `json:"resolved_at,omitempty"`
	ClosedAt             *time.Time `json:"closed_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	// Joined fields
	CreatedByUsername    *string `json:"created_by_username,omitempty"`
	CreatedByDisplayName *string `json:"created_by_display_name,omitempty"`
	AssignedToUsername   *string `json:"assigned_to_username,omitempty"`
	AssignedToDisplayName *string `json:"assigned_to_display_name,omitempty"`

	// Nested
	Comments []TicketComment `json:"comments,omitempty"`
}

type TicketComment struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticket_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`

	// Joined fields
	Username    *string `json:"username,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}
