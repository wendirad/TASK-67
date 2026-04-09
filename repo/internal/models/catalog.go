package models

import "time"

type CatalogItem struct {
	Type               string     `json:"type"`
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	Subtitle           string     `json:"subtitle"`
	Availability       string     `json:"availability"`
	AvailabilityDetail string     `json:"availability_detail"`
	ImageURL           *string    `json:"image_url"`
	PriceCents         *int       `json:"price_cents"`
	StartTime          *time.Time `json:"start_time"`
}
