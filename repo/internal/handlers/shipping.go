package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type ShippingHandler struct {
	shippingService *services.ShippingService
}

func NewShippingHandler(shippingService *services.ShippingService) *ShippingHandler {
	return &ShippingHandler{shippingService: shippingService}
}

func (h *ShippingHandler) List(c *gin.Context) {
	p := ParsePagination(c)
	status := c.Query("status")
	orderNumber := c.Query("order_number")

	records, total, err := h.shippingService.ListShipments(p.Page, p.PageSize, status, orderNumber)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if records == nil {
		records = []models.ShippingRecord{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(records, total, p))
}

type shipRequest struct {
	TrackingNumber *string `json:"tracking_number"`
	Carrier        *string `json:"carrier"`
}

func (h *ShippingHandler) Ship(c *gin.Context) {
	id := c.Param("id")
	var req shipRequest
	c.ShouldBindJSON(&req) // optional fields

	staffID := middleware.GetUserID(c)
	code, msg := h.shippingService.Ship(id, staffID, req.TrackingNumber, req.Carrier)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Shipment marked as shipped", nil)
}

type deliverRequest struct {
	ProofType string `json:"proof_type"`
	ProofData string `json:"proof_data"`
}

func (h *ShippingHandler) Deliver(c *gin.Context) {
	id := c.Param("id")
	var req deliverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "proof_type and proof_data are required")
		return
	}

	staffID := middleware.GetUserID(c)
	code, msg := h.shippingService.Deliver(id, staffID, req.ProofType, req.ProofData)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Delivery confirmed", nil)
}

type exceptionRequest struct {
	ExceptionNotes string `json:"exception_notes"`
}

func (h *ShippingHandler) Exception(c *gin.Context) {
	id := c.Param("id")
	var req exceptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "exception_notes is required")
		return
	}

	staffID := middleware.GetUserID(c)
	code, msg := h.shippingService.MarkException(id, staffID, req.ExceptionNotes)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Delivery exception recorded", nil)
}

func (h *ShippingHandler) CompleteOrder(c *gin.Context) {
	orderID := c.Param("id")
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	code, msg := h.shippingService.CompleteOrder(orderID, userID, role)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Order completed", nil)
}
