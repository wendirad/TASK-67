package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/repository"

	"github.com/gin-gonic/gin"
)

type WaitlistHandler struct {
	waitlistRepo *repository.WaitlistRepository
}

func NewWaitlistHandler(waitlistRepo *repository.WaitlistRepository) *WaitlistHandler {
	return &WaitlistHandler{waitlistRepo: waitlistRepo}
}

func (h *WaitlistHandler) GetPosition(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		Error(c, http.StatusBadRequest, "session_id query parameter is required")
		return
	}

	userID := middleware.GetUserID(c)
	pos, err := h.waitlistRepo.GetPosition(userID, sessionID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if pos == nil {
		Error(c, http.StatusNotFound, "Not on waitlist for this session")
		return
	}

	Success(c, http.StatusOK, "OK", pos)
}
