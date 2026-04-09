package services

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type ShippingService struct {
	shippingRepo *repository.ShippingRepository
	orderRepo    *repository.OrderRepository
}

func NewShippingService(shippingRepo *repository.ShippingRepository, orderRepo *repository.OrderRepository) *ShippingService {
	return &ShippingService{shippingRepo: shippingRepo, orderRepo: orderRepo}
}

// ListShipments returns paginated shipping records for staff/admin.
func (s *ShippingService) ListShipments(page, pageSize int, status, orderNumber string) ([]models.ShippingRecord, int, error) {
	return s.shippingRepo.ListAll(page, pageSize, status, orderNumber)
}

// GetShipment returns a shipping record by ID.
func (s *ShippingService) GetShipment(id string) (*models.ShippingRecord, int, string) {
	sr, err := s.shippingRepo.FindByID(id)
	if err != nil {
		log.Printf("Error finding shipping record %s: %v", id, err)
		return nil, 500, "Internal server error"
	}
	if sr == nil {
		return nil, 404, "Shipping record not found"
	}
	return sr, 200, ""
}

// Ship marks a shipment as shipped.
func (s *ShippingService) Ship(id, staffID string, trackingNumber, carrier *string) (int, string) {
	sr, err := s.shippingRepo.FindByID(id)
	if err != nil {
		log.Printf("Error finding shipping record %s: %v", id, err)
		return 500, "Internal server error"
	}
	if sr == nil {
		return 404, "Shipping record not found"
	}
	if sr.Status != "pending" {
		return 422, "Shipment is not in pending state"
	}

	if err := s.shippingRepo.Ship(id, staffID, trackingNumber, carrier); err != nil {
		log.Printf("Error shipping %s: %v", id, err)
		return 500, "Internal server error"
	}

	log.Printf("Shipment shipped: %s order=%s", id, sr.OrderID)
	return 200, ""
}

// Deliver confirms delivery with proof.
func (s *ShippingService) Deliver(id, staffID, proofType, proofData string) (int, string) {
	if proofType == "" {
		return 400, "proof_type is required"
	}
	if proofType != "signature" && proofType != "acknowledgment" {
		return 400, "proof_type must be 'signature' or 'acknowledgment'"
	}
	if proofData == "" {
		return 400, "proof_data is required"
	}
	if proofType == "signature" {
		if _, err := base64.StdEncoding.DecodeString(proofData); err != nil {
			return 400, "proof_data must be valid base64 for signature proof"
		}
	}

	sr, err := s.shippingRepo.FindByID(id)
	if err != nil {
		log.Printf("Error finding shipping record %s: %v", id, err)
		return 500, "Internal server error"
	}
	if sr == nil {
		return 404, "Shipping record not found"
	}
	if sr.Status != "shipped" && sr.Status != "in_transit" {
		return 422, "Shipment must be shipped or in transit to mark as delivered"
	}

	if err := s.shippingRepo.Deliver(id, staffID, proofType, proofData); err != nil {
		log.Printf("Error delivering %s: %v", id, err)
		return 500, "Internal server error"
	}

	log.Printf("Shipment delivered: %s order=%s proof=%s", id, sr.OrderID, proofType)
	return 200, ""
}

// MarkException marks a shipment with an exception and creates a ticket.
func (s *ShippingService) MarkException(id, staffID, notes string) (int, string) {
	if notes == "" {
		return 400, "exception_notes is required"
	}

	sr, err := s.shippingRepo.FindByID(id)
	if err != nil {
		log.Printf("Error finding shipping record %s: %v", id, err)
		return 500, "Internal server error"
	}
	if sr == nil {
		return 404, "Shipping record not found"
	}
	if sr.Status == "delivered" || sr.Status == "exception" {
		return 422, "Cannot mark exception on a delivered or already-exception shipment"
	}

	if err := s.shippingRepo.MarkException(id, staffID, notes); err != nil {
		log.Printf("Error marking exception %s: %v", id, err)
		return 500, "Internal server error"
	}

	// Create delivery exception ticket
	ticketNumber := fmt.Sprintf("TKT-%s-%05d", time.Now().Format("20060102"), time.Now().UnixNano()%100000)
	_, ticketErr := s.shippingRepo.DB().Exec(`
		INSERT INTO tickets (ticket_number, subject, type, status, related_entity_type, related_entity_id, description, created_by)
		VALUES ($1, $2, 'delivery_exception', 'open', 'shipping_record', $3, $4, $5)
	`, ticketNumber, "Delivery exception for shipment "+id, id, notes, staffID)
	if ticketErr != nil {
		log.Printf("Warning: failed to create delivery exception ticket: %v", ticketErr)
	}

	log.Printf("Shipment exception: %s order=%s", id, sr.OrderID)
	return 200, ""
}

// CompleteOrder transitions a delivered order to completed.
func (s *ShippingService) CompleteOrder(orderID, userID, role string) (int, string) {
	order, err := s.orderRepo.FindByID(orderID)
	if err != nil {
		log.Printf("Error finding order %s: %v", orderID, err)
		return 500, "Internal server error"
	}
	if order == nil {
		return 404, "Order not found"
	}
	if role != "admin" && role != "staff" && order.UserID != userID {
		return 403, "Not your order"
	}
	if order.Status != "delivered" {
		return 422, "Order must be in delivered state to complete"
	}

	if err := s.shippingRepo.CompleteOrder(orderID); err != nil {
		log.Printf("Error completing order %s: %v", orderID, err)
		return 500, "Internal server error"
	}

	log.Printf("Order completed: %s", orderID)
	return 200, ""
}
