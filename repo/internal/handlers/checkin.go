package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type CheckInHandler struct {
	checkinService *services.CheckInService
}

func NewCheckInHandler(checkinService *services.CheckInService) *CheckInHandler {
	return &CheckInHandler{checkinService: checkinService}
}

// CheckIn handles POST /api/checkin — staff-confirmed QR scan at kiosk.
func (h *CheckInHandler) CheckIn(c *gin.Context) {
	var req models.CheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "registration_id and kiosk_device_token are required")
		return
	}

	staffUserID := middleware.GetUserID(c)
	ci, code, msg := h.checkinService.PerformCheckIn(staffUserID, &req)
	if code != http.StatusCreated {
		Error(c, code, msg)
		return
	}

	Created(c, "Check-in successful", ci)
}

// Get handles GET /api/checkin/:id.
func (h *CheckInHandler) Get(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	ci, code, msg := h.checkinService.GetCheckIn(id, userID, role)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "OK", ci)
}

// StartBreak handles POST /api/checkin/:id/break.
func (h *CheckInHandler) StartBreak(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	code, msg := h.checkinService.StartBreak(id, userID, role)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Break started", nil)
}

// ReturnFromBreak handles POST /api/checkin/:id/return.
func (h *CheckInHandler) ReturnFromBreak(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	code, msg := h.checkinService.ReturnFromBreak(id, userID, role)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Returned from break", nil)
}

// GenerateQR handles GET /api/sessions/:id/qr — staff/admin only.
func (h *CheckInHandler) GenerateQR(c *gin.Context) {
	sessionID := c.Param("id")

	qr, code, msg := h.checkinService.GenerateSessionQR(sessionID)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "OK", qr)
}
