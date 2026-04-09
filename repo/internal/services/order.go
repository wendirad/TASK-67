package services

import (
	"fmt"
	"log"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type OrderService struct {
	orderRepo   *repository.OrderRepository
	productRepo *repository.ProductRepository
	addressRepo *repository.AddressRepository
	cartRepo    *repository.CartRepository
	userRepo    *repository.UserRepository
	auditRepo   *repository.AuditRepository
}

func NewOrderService(
	orderRepo *repository.OrderRepository,
	productRepo *repository.ProductRepository,
	addressRepo *repository.AddressRepository,
	cartRepo *repository.CartRepository,
	userRepo *repository.UserRepository,
	auditRepo *repository.AuditRepository,
) *OrderService {
	return &OrderService{
		orderRepo:   orderRepo,
		productRepo: productRepo,
		addressRepo: addressRepo,
		cartRepo:    cartRepo,
		userRepo:    userRepo,
		auditRepo:   auditRepo,
	}
}

// CreateOrder validates inputs, creates order with atomic stock deduction, and clears cart if source=cart.
func (s *OrderService) CreateOrder(userID string, req *models.CreateOrderRequest) (*models.Order, int, string) {
	// Check ban status
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		log.Printf("Error finding user %s: %v", userID, err)
		return nil, 500, "Internal server error"
	}
	if user == nil {
		return nil, 404, "User not found"
	}
	if user.Status == "banned" {
		return nil, 403, "Your account is banned and cannot place orders."
	}

	if len(req.Items) == 0 {
		return nil, 400, "At least one item is required"
	}

	if req.Source != "cart" && req.Source != "buy_now" {
		return nil, 400, "Source must be 'cart' or 'buy_now'"
	}

	// Validate and load all products
	products := make(map[string]*models.Product)
	hasShippable := false
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, 400, "Quantity must be greater than 0"
		}

		product, err := s.productRepo.FindByID(item.ProductID)
		if err != nil {
			log.Printf("Error finding product %s: %v", item.ProductID, err)
			return nil, 500, "Internal server error"
		}
		if product == nil {
			return nil, 404, "Product not found: " + item.ProductID
		}
		if product.Status != "active" {
			return nil, 422, "Product is not available: " + product.Name
		}
		if item.Quantity > product.StockQuantity {
			return nil, 422, "Insufficient stock for: " + product.Name
		}
		if product.IsShippable {
			hasShippable = true
		}
		products[item.ProductID] = product
	}

	// Validate shipping address if any shippable items
	var address *models.Address
	if hasShippable {
		if req.ShippingAddressID == nil || *req.ShippingAddressID == "" {
			return nil, 400, "Shipping address is required for shippable items"
		}
		addr, err := s.addressRepo.FindByID(*req.ShippingAddressID)
		if err != nil {
			log.Printf("Error finding address %s: %v", *req.ShippingAddressID, err)
			return nil, 500, "Internal server error"
		}
		if addr == nil {
			return nil, 404, "Shipping address not found"
		}
		if addr.UserID != userID {
			return nil, 403, "Shipping address does not belong to you"
		}
		address = addr
	}

	// Create order atomically (stock deduction + order + items + payment)
	order, err := s.orderRepo.CreateOrder(userID, req.Items, products, address)
	if err != nil {
		log.Printf("Error creating order: %v", err)
		return nil, 422, err.Error()
	}

	// Clear cart items if source=cart
	if req.Source == "cart" {
		productIDs := make([]string, 0, len(req.Items))
		for _, item := range req.Items {
			productIDs = append(productIDs, item.ProductID)
		}
		if err := s.cartRepo.DeleteByUserAndProducts(userID, productIDs); err != nil {
			log.Printf("Warning: failed to clear cart items after order: %v", err)
			// Non-fatal: order was created successfully
		}
	}

	log.Printf("Order created: order=%s number=%s user=%s total=%d", order.ID, order.OrderNumber, userID, order.TotalCents)

	newVal := fmt.Sprintf(`{"order_number":"%s","total_cents":%d,"status":"pending_payment"}`, order.OrderNumber, order.TotalCents)
	if err := s.auditRepo.Log("order", order.ID, "order_created", nil, &newVal, userID, ""); err != nil {
		log.Printf("Warning: failed to create audit log for order %s: %v", order.ID, err)
	}

	return order, 201, ""
}

// GetOrder returns order details with ownership check.
func (s *OrderService) GetOrder(orderID, userID, role string) (*models.Order, int, string) {
	order, err := s.orderRepo.FindByID(orderID)
	if err != nil {
		log.Printf("Error finding order %s: %v", orderID, err)
		return nil, 500, "Internal server error"
	}
	if order == nil {
		return nil, 404, "Order not found"
	}

	if role != "admin" && role != "staff" && order.UserID != userID {
		return nil, 403, "Not your order"
	}

	return order, 200, ""
}

// ListOrders returns paginated orders for member or admin/staff.
func (s *OrderService) ListOrders(userID, role string, page, pageSize int, status string) ([]models.Order, int, error) {
	if role == "admin" || role == "staff" {
		return s.orderRepo.ListAll(page, pageSize, status, "")
	}
	return s.orderRepo.ListByUser(userID, page, pageSize, status)
}

// CancelOrder cancels a pending_payment order.
func (s *OrderService) CancelOrder(orderID, userID, role string) (int, string) {
	order, err := s.orderRepo.FindByID(orderID)
	if err != nil {
		log.Printf("Error finding order %s: %v", orderID, err)
		return 500, "Internal server error"
	}
	if order == nil {
		return 404, "Order not found"
	}

	// Members can only cancel their own pending_payment orders
	if role != "admin" {
		if order.UserID != userID {
			return 403, "Not your order"
		}
		if order.Status != "pending_payment" {
			return 422, "Can only cancel orders that are pending payment"
		}
	} else {
		if order.Status != "pending_payment" {
			return 422, "Can only cancel orders that are pending payment"
		}
	}

	if err := s.orderRepo.CancelOrder(orderID); err != nil {
		log.Printf("Error canceling order %s: %v", orderID, err)
		return 500, "Internal server error"
	}

	log.Printf("Order canceled: order=%s user=%s", orderID, userID)

	oldVal := fmt.Sprintf(`{"status":"%s"}`, order.Status)
	newVal := `{"status":"closed","close_reason":"Canceled by user"}`
	if err := s.auditRepo.Log("order", orderID, "order_canceled", &oldVal, &newVal, userID, ""); err != nil {
		log.Printf("Warning: failed to create audit log for cancel order %s: %v", orderID, err)
	}

	return 200, ""
}

// RefundOrder processes a refund (admin only).
func (s *OrderService) RefundOrder(orderID, userID string) (int, string) {
	order, err := s.orderRepo.FindByID(orderID)
	if err != nil {
		log.Printf("Error finding order %s: %v", orderID, err)
		return 500, "Internal server error"
	}
	if order == nil {
		return 404, "Order not found"
	}

	refundableStatuses := map[string]bool{
		"paid": true, "processing": true, "shipped": true, "delivered": true, "completed": true,
	}
	if !refundableStatuses[order.Status] {
		return 422, "Order cannot be refunded in current state"
	}

	if err := s.orderRepo.RefundOrder(orderID); err != nil {
		log.Printf("Error refunding order %s: %v", orderID, err)
		return 500, "Internal server error"
	}

	log.Printf("Order refunded: order=%s", orderID)

	oldVal := fmt.Sprintf(`{"status":"%s"}`, order.Status)
	newVal := `{"status":"refunded"}`
	if err := s.auditRepo.Log("order", orderID, "order_refunded", &oldVal, &newVal, userID, ""); err != nil {
		log.Printf("Warning: failed to create audit log for refund order %s: %v", orderID, err)
	}

	return 200, ""
}
