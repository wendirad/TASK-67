package unit_tests

import (
	"testing"

	"campusrec/internal/models"
)

// TestOrderCancelableStatuses calls the real models.IsOrderCancelable function
// to verify the production cancelable-status logic.
func TestOrderCancelableStatuses(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"pending_payment", true},
		{"paid", false},
		{"processing", false},
		{"shipped", false},
		{"delivered", false},
		{"completed", false},
		{"closed", false},
		{"refunded", false},
		{"refund_pending", false},
	}

	for _, tt := range tests {
		got := models.IsOrderCancelable(tt.status)
		if got != tt.expected {
			t.Errorf("IsOrderCancelable(%q) = %v, want %v", tt.status, got, tt.expected)
		}
	}
}

// TestOrderRefundableStatuses calls the real models.IsOrderRefundable function
// to verify the production refundable-status logic.
func TestOrderRefundableStatuses(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"paid", true},
		{"processing", true},
		{"shipped", true},
		{"delivered", true},
		{"completed", true},
		{"pending_payment", false},
		{"closed", false},
		{"refunded", false},
		{"refund_pending", false},
	}

	for _, tt := range tests {
		got := models.IsOrderRefundable(tt.status)
		if got != tt.expected {
			t.Errorf("IsOrderRefundable(%q) = %v, want %v", tt.status, got, tt.expected)
		}
	}
}

// TestOrderTotalCalculation calls the real models.OrderTotalCents function
// to verify the production total-calculation logic used in order creation.
func TestOrderTotalCalculation(t *testing.T) {
	products := map[string]*models.Product{
		"p1": {PriceCents: 1000},
		"p2": {PriceCents: 500},
		"p3": {PriceCents: 2500},
	}

	items := []models.CreateOrderItem{
		{ProductID: "p1", Quantity: 2},  // 1000 * 2 = 2000
		{ProductID: "p2", Quantity: 3},  // 500  * 3 = 1500
		{ProductID: "p3", Quantity: 1},  // 2500 * 1 = 2500
	}

	got := models.OrderTotalCents(items, products)
	expected := 6000
	if got != expected {
		t.Errorf("OrderTotalCents = %d, want %d", got, expected)
	}
}

// TestOrderTotalSingleItem verifies total for a single-item order.
func TestOrderTotalSingleItem(t *testing.T) {
	products := map[string]*models.Product{
		"p1": {PriceCents: 3999},
	}
	items := []models.CreateOrderItem{
		{ProductID: "p1", Quantity: 1},
	}

	got := models.OrderTotalCents(items, products)
	if got != 3999 {
		t.Errorf("OrderTotalCents = %d, want 3999", got)
	}
}

// TestOrderTotalEmptyItems verifies total is zero when no items are provided.
func TestOrderTotalEmptyItems(t *testing.T) {
	products := map[string]*models.Product{}
	items := []models.CreateOrderItem{}

	got := models.OrderTotalCents(items, products)
	if got != 0 {
		t.Errorf("OrderTotalCents with no items = %d, want 0", got)
	}
}

// TestOrderCancelableExcludesAllNonPending verifies that every known
// non-pending_payment status is not cancelable.
func TestOrderCancelableExcludesAllNonPending(t *testing.T) {
	allStatuses := []string{
		"created", "pending_payment", "paid", "processing",
		"shipped", "delivered", "completed", "closed",
		"refund_pending", "refunded",
	}

	for _, status := range allStatuses {
		got := models.IsOrderCancelable(status)
		want := status == "pending_payment"
		if got != want {
			t.Errorf("IsOrderCancelable(%q) = %v, want %v", status, got, want)
		}
	}
}

// TestOrderRefundableExcludesTerminalStatuses verifies terminal/non-paid
// statuses are never refundable.
func TestOrderRefundableExcludesTerminalStatuses(t *testing.T) {
	terminal := []string{"pending_payment", "created", "closed", "refunded", "refund_pending"}
	for _, status := range terminal {
		if models.IsOrderRefundable(status) {
			t.Errorf("IsOrderRefundable(%q) should be false for terminal/non-paid status", status)
		}
	}
}
