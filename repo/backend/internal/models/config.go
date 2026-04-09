package models

import "time"

type ConfigEntry struct {
	ID               string    `json:"id"`
	Key              string    `json:"key"`
	Value            string    `json:"value"`
	Description      *string   `json:"description,omitempty"`
	CanaryPercentage *int      `json:"canary_percentage,omitempty"`
	UpdatedBy        *string   `json:"updated_by,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type AuditLog struct {
	ID          string    `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	Action      string    `json:"action"`
	OldValue    *string   `json:"old_value,omitempty"`
	NewValue    *string   `json:"new_value,omitempty"`
	PerformedBy *string   `json:"performed_by,omitempty"`
	IPAddress   *string   `json:"ip_address,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
