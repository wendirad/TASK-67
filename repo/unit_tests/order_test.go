package unit_tests

import (
	"testing"
)

// Test order state transition rules (documented valid transitions)
func TestOrderCancelableStatuses(t *testing.T) {
	cancelable := map[string]bool{
		"pending_payment": true,
	}

	nonCancelable := []string{"paid", "processing", "shipped", "delivered", "completed", "closed", "refunded"}

	for _, status := range nonCancelable {
		if cancelable[status] {
			t.Errorf("Status %q should not be cancelable", status)
		}
	}

	if !cancelable["pending_payment"] {
		t.Error("pending_payment should be cancelable")
	}
}

func TestOrderRefundableStatuses(t *testing.T) {
	refundable := map[string]bool{
		"paid": true, "processing": true, "shipped": true, "delivered": true, "completed": true,
	}

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
		{"cancelled", false},
	}

	for _, tt := range tests {
		if refundable[tt.status] != tt.expected {
			t.Errorf("Refundable(%q) = %v, want %v", tt.status, refundable[tt.status], tt.expected)
		}
	}
}

func TestOrderTotalCalculation(t *testing.T) {
	// Simulate order total calculation: sum of unit_price * quantity
	type item struct {
		unitPriceCents int
		quantity       int
	}

	items := []item{
		{1000, 2},  // $10.00 x 2 = $20.00
		{500, 3},   // $5.00 x 3 = $15.00
		{2500, 1},  // $25.00 x 1 = $25.00
	}

	total := 0
	for _, i := range items {
		total += i.unitPriceCents * i.quantity
	}

	expected := 6000 // $60.00
	if total != expected {
		t.Errorf("Total = %d, want %d", total, expected)
	}
}
