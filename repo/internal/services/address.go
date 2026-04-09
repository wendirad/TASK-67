package services

import (
	"fmt"
	"log"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type AddressService struct {
	addressRepo *repository.AddressRepository
}

func NewAddressService(addressRepo *repository.AddressRepository) *AddressService {
	return &AddressService{addressRepo: addressRepo}
}

func (s *AddressService) ListAddresses(userID string) ([]models.Address, error) {
	return s.addressRepo.ListByUser(userID)
}

type AddressInput struct {
	Label         string  `json:"label"`
	RecipientName string  `json:"recipient_name"`
	Phone         string  `json:"phone"`
	AddressLine1  string  `json:"address_line1"`
	AddressLine2  *string `json:"address_line2"`
	City          string  `json:"city"`
	Province      string  `json:"province"`
	PostalCode    string  `json:"postal_code"`
	IsDefault     bool    `json:"is_default"`
}

// CreateAddress creates a new address for the user.
// Returns (address, httpCode, errorMsg).
func (s *AddressService) CreateAddress(userID string, input AddressInput) (*models.Address, int, string) {
	count, err := s.addressRepo.CountByUser(userID)
	if err != nil {
		log.Printf("Error counting addresses for user %s: %v", userID, err)
		return nil, 500, "Internal server error"
	}
	if count >= 10 {
		return nil, 422, "Maximum of 10 addresses per user reached"
	}

	addr := &models.Address{
		UserID:        userID,
		Label:         input.Label,
		RecipientName: input.RecipientName,
		Phone:         input.Phone,
		AddressLine1:  input.AddressLine1,
		AddressLine2:  input.AddressLine2,
		City:          input.City,
		Province:      input.Province,
		PostalCode:    input.PostalCode,
		IsDefault:     input.IsDefault,
	}

	if count == 0 {
		addr.IsDefault = true
	}

	if err := s.addressRepo.Create(addr); err != nil {
		log.Printf("Error creating address for user %s: %v", userID, err)
		return nil, 500, "Internal server error"
	}

	return addr, 201, ""
}

// UpdateAddress updates an existing address.
// Returns (address, httpCode, errorMsg).
func (s *AddressService) UpdateAddress(userID, addressID string, input AddressInput) (*models.Address, int, string) {
	existing, err := s.addressRepo.FindByID(addressID)
	if err != nil {
		log.Printf("Error finding address %s: %v", addressID, err)
		return nil, 500, "Internal server error"
	}
	if existing == nil || existing.UserID != userID {
		return nil, 404, "Address not found"
	}

	existing.Label = input.Label
	existing.RecipientName = input.RecipientName
	existing.Phone = input.Phone
	existing.AddressLine1 = input.AddressLine1
	existing.AddressLine2 = input.AddressLine2
	existing.City = input.City
	existing.Province = input.Province
	existing.PostalCode = input.PostalCode
	existing.IsDefault = input.IsDefault

	if err := s.addressRepo.Update(existing); err != nil {
		log.Printf("Error updating address %s: %v", addressID, err)
		return nil, 500, "Internal server error"
	}

	return existing, 200, ""
}

// DeleteAddress deletes an address if not in use by active orders.
// Returns (httpCode, errorMsg).
func (s *AddressService) DeleteAddress(userID, addressID string) (int, string) {
	existing, err := s.addressRepo.FindByID(addressID)
	if err != nil {
		log.Printf("Error finding address %s: %v", addressID, err)
		return 500, "Internal server error"
	}
	if existing == nil || existing.UserID != userID {
		return 404, "Address not found"
	}

	inUse, err := s.addressRepo.IsAddressInUse(addressID)
	if err != nil {
		log.Printf("Error checking address usage %s: %v", addressID, err)
		return 500, "Internal server error"
	}
	if inUse {
		return 409, "Address is in use by an active order"
	}

	if err := s.addressRepo.Delete(addressID, userID); err != nil {
		log.Printf("Error deleting address %s: %v", addressID, err)
		return 500, "Internal server error"
	}

	return 200, ""
}

// SetDefault sets an address as the user's default.
func (s *AddressService) SetDefault(userID, addressID string) (int, string) {
	existing, err := s.addressRepo.FindByID(addressID)
	if err != nil {
		log.Printf("Error finding address %s: %v", addressID, err)
		return 500, "Internal server error"
	}
	if existing == nil || existing.UserID != userID {
		return 404, "Address not found"
	}

	if err := s.addressRepo.SetDefault(addressID, userID); err != nil {
		log.Printf("Error setting default address %s: %v", addressID, err)
		return 500, fmt.Sprintf("Internal server error")
	}

	return 200, ""
}
