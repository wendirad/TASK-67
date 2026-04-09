package handlers

import (
	"net/http"

	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminRegistrationHandler struct {
	regService *services.RegistrationService
}

func NewAdminRegistrationHandler(regService *services.RegistrationService) *AdminRegistrationHandler {
	return &AdminRegistrationHandler{regService: regService}
}

func (h *AdminRegistrationHandler) List(c *gin.Context) {
	p := ParsePagination(c)
	sessionID := c.Query("session_id")
	userID := c.Query("user_id")
	status := c.Query("status")

	regs, total, err := h.regService.ListAllRegistrations(p.Page, p.PageSize, sessionID, userID, status)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if regs == nil {
		regs = []models.Registration{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(regs, total, p))
}

func (h *AdminRegistrationHandler) Approve(c *gin.Context) {
	regID := c.Param("id")

	reg, code, msg := h.regService.ApproveRegistration(regID)
	if reg == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Registration approved", reg)
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminRegistrationHandler) Reject(c *gin.Context) {
	regID := c.Param("id")

	var req rejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	reg, code, msg := h.regService.RejectRegistration(regID, req.Reason)
	if reg == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Registration rejected", reg)
}
