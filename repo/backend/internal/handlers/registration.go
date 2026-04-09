package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type RegistrationHandler struct {
	regService *services.RegistrationService
}

func NewRegistrationHandler(regService *services.RegistrationService) *RegistrationHandler {
	return &RegistrationHandler{regService: regService}
}

type createRegistrationRequest struct {
	SessionID string `json:"session_id"`
}

func (h *RegistrationHandler) Create(c *gin.Context) {
	var req createRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.SessionID == "" {
		Error(c, http.StatusBadRequest, "Session ID is required")
		return
	}

	userID := middleware.GetUserID(c)
	reg, code, msg := h.regService.CreateRegistration(userID, req.SessionID)
	if reg == nil {
		Error(c, code, msg)
		return
	}

	Created(c, "Registration created", reg)
}

func (h *RegistrationHandler) List(c *gin.Context) {
	p := ParsePagination(c)
	userID := middleware.GetUserID(c)
	status := c.Query("status")

	regs, total, err := h.regService.ListUserRegistrations(userID, p.Page, p.PageSize, status)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if regs == nil {
		regs = []models.Registration{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(regs, total, p))
}

func (h *RegistrationHandler) Confirm(c *gin.Context) {
	regID := c.Param("id")
	userID := middleware.GetUserID(c)

	reg, code, msg := h.regService.ConfirmRegistration(regID, userID)
	if reg == nil {
		Error(c, code, msg)
		return
	}

	if reg.Status == "waitlisted" {
		Success(c, http.StatusOK, msg, reg)
		return
	}

	Success(c, http.StatusOK, "Registration confirmed", reg)
}

func (h *RegistrationHandler) Cancel(c *gin.Context) {
	regID := c.Param("id")
	userID := middleware.GetUserID(c)
	isAdmin := middleware.GetUserRole(c) == "admin"

	reg, code, msg := h.regService.CancelRegistration(regID, userID, isAdmin)
	if reg == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Registration canceled", reg)
}
