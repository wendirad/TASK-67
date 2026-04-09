package models

import "time"

type CartItem struct {
	ID            string    `json:"id"`
	UserID        string    `json:"-"`
	ProductID     string    `json:"product_id"`
	Quantity      int       `json:"quantity"`
	AddedAt       time.Time `json:"added_at"`
	SubtotalCents int       `json:"subtotal_cents"`

	// Joined product fields
	Product *CartProductInfo `json:"product,omitempty"`
}

type CartProductInfo struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	PriceCents    int     `json:"price_cents"`
	StockQuantity int     `json:"stock_quantity"`
	ImageURL      *string `json:"image_url"`
	IsShippable   bool    `json:"is_shippable"`
	Status        string  `json:"status"`
}

type CartResponse struct {
	Items      []CartItem `json:"items"`
	TotalCents int        `json:"total_cents"`
	ItemCount  int        `json:"item_count"`
}
