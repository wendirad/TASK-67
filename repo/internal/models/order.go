package models

import "time"

type Order struct {
	ID                string     `json:"id"`
	OrderNumber       string     `json:"order_number"`
	UserID            string     `json:"user_id"`
	Status            string     `json:"status"`
	TotalCents        int        `json:"total_cents"`
	ShippingAddressID *string    `json:"shipping_address_id,omitempty"`
	ShipToRecipient   *string    `json:"ship_to_recipient,omitempty"`
	ShipToPhone       *string    `json:"ship_to_phone,omitempty"`
	ShipToLine1       *string    `json:"ship_to_line1,omitempty"`
	ShipToLine2       *string    `json:"ship_to_line2,omitempty"`
	ShipToCity        *string    `json:"ship_to_city,omitempty"`
	ShipToProvince    *string    `json:"ship_to_province,omitempty"`
	ShipToPostalCode  *string    `json:"ship_to_postal_code,omitempty"`
	PaymentDeadline   *time.Time `json:"payment_deadline,omitempty"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	CloseReason       *string    `json:"close_reason,omitempty"`
	Notes             *string    `json:"notes,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	// Joined fields
	Items   []OrderItem `json:"items,omitempty"`
	Payment *Payment    `json:"payment,omitempty"`

	// Joined user info (admin list)
	Username    *string `json:"username,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}

type OrderItem struct {
	ID             string `json:"id"`
	OrderID        string `json:"-"`
	ProductID      string `json:"product_id"`
	ProductName    string `json:"product_name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int    `json:"unit_price_cents"`
	TotalCents     int    `json:"total_cents"`
}

type Payment struct {
	ID                 string     `json:"id"`
	OrderID            string     `json:"-"`
	PaymentMethod      string     `json:"payment_method"`
	AmountCents        int        `json:"amount_cents"`
	Status             string     `json:"status"`
	TransactionID      *string    `json:"transaction_id,omitempty"`
	WeChatPrepayData   *string    `json:"wechat_prepay_data,omitempty"`
	CallbackSignature  *string    `json:"-"`
	CallbackReceivedAt *time.Time `json:"callback_received_at,omitempty"`
	RefundID           *string    `json:"refund_id,omitempty"`
	RefundedAt         *time.Time `json:"refunded_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CreateOrderRequest struct {
	Items             []CreateOrderItem `json:"items"`
	ShippingAddressID *string           `json:"shipping_address_id"`
	Source            string            `json:"source"`
}

type CreateOrderItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// IsOrderCancelable returns true if an order in the given status can be canceled.
func IsOrderCancelable(status string) bool {
	return status == "pending_payment"
}

// IsOrderRefundable returns true if an order in the given status can be refunded.
func IsOrderRefundable(status string) bool {
	switch status {
	case "paid", "processing", "shipped", "delivered", "completed":
		return true
	}
	return false
}

// OrderTotalCents computes the order total from a slice of items and a product
// lookup map keyed by product ID.
func OrderTotalCents(items []CreateOrderItem, products map[string]*Product) int {
	total := 0
	for _, item := range items {
		if p, ok := products[item.ProductID]; ok {
			total += p.PriceCents * item.Quantity
		}
	}
	return total
}
