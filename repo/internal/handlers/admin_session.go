package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminSessionHandler struct {
	sessionService *services.SessionService
}

func NewAdminSessionHandler(sessionService *services.SessionService) *AdminSessionHandler {
	return &AdminSessionHandler{sessionService: sessionService}
}

var validSessionStatuses = map[string]bool{
	"open":     true,
	"closed":   true,
	"canceled": true,
}

func (h *AdminSessionHandler) Create(c *gin.Context) {
	var input services.CreateSessionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Title == "" {
		Error(c, http.StatusBadRequest, "Title is required")
		return
	}
	if input.FacilityID == "" {
		Error(c, http.StatusBadRequest, "Facility ID is required")
		return
	}
	if input.StartTime.IsZero() {
		Error(c, http.StatusBadRequest, "Start time is required")
		return
	}
	if input.EndTime.IsZero() {
		Error(c, http.StatusBadRequest, "End time is required")
		return
	}
	if !input.EndTime.After(input.StartTime) {
		Error(c, http.StatusBadRequest, "End time must be after start time")
		return
	}
	if input.TotalSeats <= 0 {
		Error(c, http.StatusBadRequest, "Total seats must be greater than 0")
		return
	}
	if input.RegistrationCloseBeforeMin < 0 {
		Error(c, http.StatusBadRequest, "Registration close before minutes must be >= 0")
		return
	}

	createdBy := middleware.GetUserID(c)
	session, code, msg := h.sessionService.CreateSession(input, createdBy)
	if session == nil {
		Error(c, code, msg)
		return
	}

	Created(c, "Session created successfully", session)
}

func (h *AdminSessionHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var input services.UpdateSessionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.TotalSeats != nil && *input.TotalSeats <= 0 {
		Error(c, http.StatusBadRequest, "Total seats must be greater than 0")
		return
	}
	if input.RegistrationCloseBeforeMin != nil && *input.RegistrationCloseBeforeMin < 0 {
		Error(c, http.StatusBadRequest, "Registration close before minutes must be >= 0")
		return
	}

	session, code, msg := h.sessionService.UpdateSession(id, input)
	if session == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Session updated successfully", session)
}

func (h *AdminSessionHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")

	var input services.UpdateSessionStatusInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !validSessionStatuses[input.Status] {
		Error(c, http.StatusBadRequest, "Invalid status. Must be one of: open, closed, canceled")
		return
	}

	session, code, msg := h.sessionService.UpdateSessionStatus(id, input.Status)
	if session == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Session status updated", session)
}
