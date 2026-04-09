package services

import (
	"log"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type CartService struct {
	cartRepo    *repository.CartRepository
	productRepo *repository.ProductRepository
}

func NewCartService(cartRepo *repository.CartRepository, productRepo *repository.ProductRepository) *CartService {
	return &CartService{cartRepo: cartRepo, productRepo: productRepo}
}

// GetCart returns the user's cart with computed totals.
func (s *CartService) GetCart(userID string) (*models.CartResponse, error) {
	items, err := s.cartRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.CartItem{}
	}

	totalCents := 0
	for _, item := range items {
		totalCents += item.SubtotalCents
	}

	return &models.CartResponse{
		Items:      items,
		TotalCents: totalCents,
		ItemCount:  len(items),
	}, nil
}

// AddToCart adds a product to the user's cart after validating the product.
func (s *CartService) AddToCart(userID, productID string, quantity int) (int, string) {
	if quantity <= 0 {
		return 400, "Quantity must be greater than 0"
	}

	product, err := s.productRepo.FindByID(productID)
	if err != nil {
		log.Printf("Error finding product %s: %v", productID, err)
		return 500, "Internal server error"
	}
	if product == nil {
		return 404, "Product not found"
	}
	if product.Status != "active" {
		return 422, "Product is not available"
	}

	// Check existing quantity in cart to validate total
	existing, err := s.cartRepo.FindByUserAndProduct(userID, productID)
	if err != nil {
		log.Printf("Error checking existing cart item: %v", err)
		return 500, "Internal server error"
	}

	totalQuantity := quantity
	if existing != nil {
		totalQuantity += existing.Quantity
	}

	if totalQuantity > product.StockQuantity {
		return 422, "Insufficient stock"
	}

	if _, err := s.cartRepo.AddOrUpdate(userID, productID, quantity); err != nil {
		log.Printf("Error adding to cart: %v", err)
		return 500, "Internal server error"
	}

	log.Printf("Cart updated: user=%s product=%s quantity_added=%d", userID, productID, quantity)
	return 201, ""
}

// UpdateQuantity updates the quantity of a cart item.
func (s *CartService) UpdateQuantity(userID, cartItemID string, quantity int) (int, string) {
	if quantity <= 0 {
		return 400, "Quantity must be greater than 0"
	}

	item, err := s.cartRepo.FindByID(cartItemID)
	if err != nil {
		log.Printf("Error finding cart item %s: %v", cartItemID, err)
		return 500, "Internal server error"
	}
	if item == nil {
		return 404, "Cart item not found"
	}
	if item.UserID != userID {
		return 403, "Not your cart item"
	}

	product, err := s.productRepo.FindByID(item.ProductID)
	if err != nil {
		log.Printf("Error finding product %s: %v", item.ProductID, err)
		return 500, "Internal server error"
	}
	if product == nil || product.Status != "active" {
		return 422, "Product is no longer available"
	}
	if quantity > product.StockQuantity {
		return 422, "Insufficient stock"
	}

	if err := s.cartRepo.UpdateQuantity(cartItemID, quantity); err != nil {
		log.Printf("Error updating cart quantity: %v", err)
		return 500, "Internal server error"
	}

	log.Printf("Cart item updated: id=%s quantity=%d", cartItemID, quantity)
	return 200, ""
}

// RemoveItem removes an item from the user's cart.
func (s *CartService) RemoveItem(userID, cartItemID string) (int, string) {
	item, err := s.cartRepo.FindByID(cartItemID)
	if err != nil {
		log.Printf("Error finding cart item %s: %v", cartItemID, err)
		return 500, "Internal server error"
	}
	if item == nil {
		return 404, "Cart item not found"
	}
	if item.UserID != userID {
		return 403, "Not your cart item"
	}

	if err := s.cartRepo.Delete(cartItemID); err != nil {
		log.Printf("Error deleting cart item: %v", err)
		return 500, "Internal server error"
	}

	log.Printf("Cart item removed: id=%s user=%s", cartItemID, userID)
	return 200, ""
}
