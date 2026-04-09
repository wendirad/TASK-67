package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paymentService *services.PaymentService
}

func NewPaymentHandler(paymentService *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

// Callback handles the payment provider callback (POST /api/payments/callback).
func (h *PaymentHandler) Callback(c *gin.Context) {
	var req services.PaymentCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid callback payload")
		return
	}

	code, msg := h.paymentService.ProcessCallback(&req)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	if msg == "Payment already confirmed" {
		Success(c, http.StatusOK, msg, nil)
		return
	}

	Success(c, http.StatusOK, "Payment confirmed", nil)
}

// SimulateCallback generates and processes a valid callback for an order (POST /api/payments/:id/simulate-callback).
// Staff/Admin only — for testing and demo purposes.
func (h *PaymentHandler) SimulateCallback(c *gin.Context) {
	orderID := c.Param("id")
	userID := middleware.GetUserID(c)

	code, msg := h.paymentService.SimulateCallback(orderID, userID)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	if msg == "Order is already paid" {
		Success(c, http.StatusOK, msg, nil)
		return
	}

	Success(c, http.StatusOK, "Payment simulated and confirmed", nil)
}
