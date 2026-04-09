package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type AddressHandler struct {
	addressService *services.AddressService
}

func NewAddressHandler(addressService *services.AddressService) *AddressHandler {
	return &AddressHandler{addressService: addressService}
}

func (h *AddressHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	addresses, err := h.addressService.ListAddresses(userID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if addresses == nil {
		addresses = []models.Address{}
	}
	Success(c, http.StatusOK, "OK", gin.H{"items": addresses})
}

func (h *AddressHandler) Create(c *gin.Context) {
	var input services.AddressInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if msg := validateAddressInput(input); msg != "" {
		Error(c, http.StatusBadRequest, msg)
		return
	}

	userID := middleware.GetUserID(c)
	addr, code, msg := h.addressService.CreateAddress(userID, input)
	if addr == nil {
		Error(c, code, msg)
		return
	}

	Created(c, "Address created successfully", addr)
}

func (h *AddressHandler) Update(c *gin.Context) {
	addressID := c.Param("id")

	var input services.AddressInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if msg := validateAddressInput(input); msg != "" {
		Error(c, http.StatusBadRequest, msg)
		return
	}

	userID := middleware.GetUserID(c)
	addr, code, msg := h.addressService.UpdateAddress(userID, addressID, input)
	if addr == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Address updated successfully", addr)
}

func (h *AddressHandler) Delete(c *gin.Context) {
	addressID := c.Param("id")
	userID := middleware.GetUserID(c)

	code, msg := h.addressService.DeleteAddress(userID, addressID)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Address deleted successfully", nil)
}

func (h *AddressHandler) SetDefault(c *gin.Context) {
	addressID := c.Param("id")
	userID := middleware.GetUserID(c)

	code, msg := h.addressService.SetDefault(userID, addressID)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Default address updated", nil)
}

func validateAddressInput(input services.AddressInput) string {
	if input.Label == "" {
		return "Label is required"
	}
	if len(input.Label) > 100 {
		return "Label must be at most 100 characters"
	}
	if input.RecipientName == "" {
		return "Recipient name is required"
	}
	if input.Phone == "" {
		return "Phone is required"
	}
	if !ValidatePhone(input.Phone) {
		return "Invalid phone number format"
	}
	if input.AddressLine1 == "" {
		return "Address line 1 is required"
	}
	if input.City == "" {
		return "City is required"
	}
	if input.Province == "" {
		return "Province is required"
	}
	if input.PostalCode == "" {
		return "Postal code is required"
	}
	if !ValidatePostalCode(input.PostalCode) {
		return "Invalid postal code format"
	}
	return ""
}
