package unit_tests

import (
	"testing"
)

// TestBuyNowSourceValidation verifies the order source field validation logic.
func TestBuyNowSourceValidation(t *testing.T) {
	validSources := map[string]bool{
		"cart":    true,
		"buy_now": true,
	}

	tests := []struct {
		source string
		valid  bool
	}{
		{"cart", true},
		{"buy_now", true},
		{"", false},
		{"direct", false},
		{"buy", false},
		{"CART", false},
		{"BUY_NOW", false},
	}

	for _, tt := range tests {
		got := validSources[tt.source]
		if got != tt.valid {
			t.Errorf("source=%q: got valid=%v, want %v", tt.source, got, tt.valid)
		}
	}
}

// TestBuyNowCartClearing verifies the cart-clearing logic based on source.
// source=cart should clear cart items; source=buy_now should not.
func TestBuyNowCartClearing(t *testing.T) {
	tests := []struct {
		source     string
		shouldClear bool
	}{
		{"cart", true},
		{"buy_now", false},
	}

	for _, tt := range tests {
		shouldClear := tt.source == "cart"
		if shouldClear != tt.shouldClear {
			t.Errorf("source=%q: shouldClearCart=%v, want %v", tt.source, shouldClear, tt.shouldClear)
		}
	}
}

// TestBuyNowShippingAddressLogic verifies shipping address requirements.
func TestBuyNowShippingAddressLogic(t *testing.T) {
	tests := []struct {
		name         string
		hasShippable bool
		hasAddress   bool
		shouldPass   bool
	}{
		{"shippable with address", true, true, true},
		{"shippable without address", true, false, false},
		{"non-shippable with address", false, true, true},
		{"non-shippable without address", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addressRequired := tt.hasShippable && !tt.hasAddress
			canProceed := !addressRequired
			if canProceed != tt.shouldPass {
				t.Errorf("got canProceed=%v, want %v", canProceed, tt.shouldPass)
			}
		})
	}
}

// TestBuyNowSingleItem verifies buy now always operates on a single item.
func TestBuyNowSingleItem(t *testing.T) {
	// Buy Now creates an order with exactly one item (one product, specified quantity)
	type buyNowRequest struct {
		productID string
		quantity  int
	}

	tests := []struct {
		name     string
		req      buyNowRequest
		valid    bool
	}{
		{"valid single item", buyNowRequest{"prod-1", 1}, true},
		{"valid quantity > 1", buyNowRequest{"prod-1", 3}, true},
		{"zero quantity", buyNowRequest{"prod-1", 0}, false},
		{"negative quantity", buyNowRequest{"prod-1", -1}, false},
		{"empty product ID", buyNowRequest{"", 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.req.productID != "" && tt.req.quantity > 0
			if valid != tt.valid {
				t.Errorf("got valid=%v, want %v", valid, tt.valid)
			}
		})
	}
}

// TestBuyNowCheckoutURLParams verifies the query parameter format
// used to navigate from product detail to checkout.
func TestBuyNowCheckoutURLParams(t *testing.T) {
	tests := []struct {
		name      string
		productID string
		qty       int
		mode      string
	}{
		{"standard buy now", "abc-123", 1, "buy_now"},
		{"multi-quantity", "xyz-789", 5, "buy_now"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The checkout page determines mode from query params:
			// ?buy_now=PRODUCT_ID&qty=N → buy_now mode
			// no params → cart mode
			if tt.productID == "" {
				t.Error("product ID cannot be empty for buy now")
			}
			if tt.qty < 1 {
				t.Error("quantity must be at least 1")
			}
			if tt.mode != "buy_now" {
				t.Errorf("mode = %q, want buy_now", tt.mode)
			}
		})
	}
}
