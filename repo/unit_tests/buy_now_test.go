package unit_tests

import (
	"testing"

	"campusrec/internal/models"
)

// TestBuyNowOrderTotalCents verifies the real OrderTotalCents computation
// used when creating orders (both buy_now and cart source).
func TestBuyNowOrderTotalCents(t *testing.T) {
	products := map[string]*models.Product{
		"prod-1": {ID: "prod-1", PriceCents: 1000},
		"prod-2": {ID: "prod-2", PriceCents: 2500},
	}

	tests := []struct {
		name  string
		items []models.CreateOrderItem
		want  int
	}{
		{
			"single item quantity 1",
			[]models.CreateOrderItem{{ProductID: "prod-1", Quantity: 1}},
			1000,
		},
		{
			"single item quantity 3",
			[]models.CreateOrderItem{{ProductID: "prod-1", Quantity: 3}},
			3000,
		},
		{
			"multiple items",
			[]models.CreateOrderItem{
				{ProductID: "prod-1", Quantity: 2},
				{ProductID: "prod-2", Quantity: 1},
			},
			4500,
		},
		{
			"empty items",
			[]models.CreateOrderItem{},
			0,
		},
		{
			"unknown product ignored",
			[]models.CreateOrderItem{{ProductID: "unknown", Quantity: 1}},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.OrderTotalCents(tt.items, products)
			if got != tt.want {
				t.Errorf("OrderTotalCents = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestBuyNowOrderCancelability verifies that only pending_payment orders
// can be canceled — the real IsOrderCancelable function used in the cancel flow.
func TestBuyNowOrderCancelability(t *testing.T) {
	tests := []struct {
		status     string
		cancelable bool
	}{
		{"pending_payment", true},
		{"paid", false},
		{"processing", false},
		{"shipped", false},
		{"delivered", false},
		{"completed", false},
		{"closed", false},
		{"refunded", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := models.IsOrderCancelable(tt.status)
			if got != tt.cancelable {
				t.Errorf("IsOrderCancelable(%q) = %v, want %v", tt.status, got, tt.cancelable)
			}
		})
	}
}

// TestBuyNowOrderRefundability verifies which order states allow refunds —
// the real IsOrderRefundable function used in the admin refund flow.
func TestBuyNowOrderRefundability(t *testing.T) {
	tests := []struct {
		status     string
		refundable bool
	}{
		{"pending_payment", false},
		{"paid", true},
		{"processing", true},
		{"shipped", true},
		{"delivered", true},
		{"completed", true},
		{"closed", false},
		{"refunded", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := models.IsOrderRefundable(tt.status)
			if got != tt.refundable {
				t.Errorf("IsOrderRefundable(%q) = %v, want %v", tt.status, got, tt.refundable)
			}
		})
	}
}

// TestBuyNowProductAvailability verifies the real ComputeAvailability method
// used to display stock status on the product detail / buy-now page.
func TestBuyNowProductAvailability(t *testing.T) {
	tests := []struct {
		name  string
		stock int
		want  string
	}{
		{"plenty in stock", 100, "in_stock"},
		{"exactly 11", 11, "in_stock"},
		{"low stock boundary", 10, "low_stock"},
		{"single remaining", 1, "low_stock"},
		{"out of stock", 0, "out_of_stock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &models.Product{StockQuantity: tt.stock}
			got := p.ComputeAvailability()
			if got != tt.want {
				t.Errorf("ComputeAvailability() = %q, want %q", got, tt.want)
			}
		})
	}
}
