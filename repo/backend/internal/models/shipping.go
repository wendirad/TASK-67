package models

import "time"

type ShippingRecord struct {
	ID             string     `json:"id"`
	OrderID        string     `json:"order_id"`
	TrackingNumber *string    `json:"tracking_number"`
	Carrier        *string    `json:"carrier"`
	Status         string     `json:"status"`
	ShippedAt      *time.Time `json:"shipped_at,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	ProofType      *string    `json:"proof_type,omitempty"`
	ProofData      *string    `json:"proof_data,omitempty"`
	ExceptionNotes *string    `json:"exception_notes,omitempty"`
	HandledBy      *string    `json:"handled_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Joined fields
	OrderNumber *string `json:"order_number,omitempty"`
	Username    *string `json:"username,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}
