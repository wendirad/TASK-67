package models

import "time"

type Address struct {
	ID            string    `json:"id"`
	UserID        string    `json:"-"`
	Label         string    `json:"label"`
	RecipientName string    `json:"recipient_name"`
	Phone         string    `json:"phone"`
	AddressLine1  string    `json:"address_line1"`
	AddressLine2  *string   `json:"address_line2"`
	City          string    `json:"city"`
	Province      string    `json:"province"`
	PostalCode    string    `json:"postal_code"`
	IsDefault     bool      `json:"is_default"`
	CreatedAt     time.Time `json:"created_at"`
}
