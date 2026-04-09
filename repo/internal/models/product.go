package models

import "time"

type Product struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   *string   `json:"description"`
	Category      string    `json:"category"`
	PriceCents    int       `json:"price_cents"`
	StockQuantity int       `json:"stock_quantity"`
	IsShippable   bool      `json:"is_shippable"`
	ImageURL      *string   `json:"image_url"`
	Status        string    `json:"status"`
	Availability  string    `json:"availability,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (p *Product) ComputeAvailability() string {
	if p.StockQuantity > 10 {
		return "in_stock"
	}
	if p.StockQuantity > 0 {
		return "low_stock"
	}
	return "out_of_stock"
}
