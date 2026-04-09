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
	userID := middleware.GetUserID(c)
	sessionID := c.Query("session_id")

	if sessionID != "" {
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
		return
	}

	// No session_id: return user's most recent active waitlist position
	pos, err := h.waitlistRepo.GetActivePosition(userID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if pos == nil {
		Success(c, http.StatusOK, "OK", nil)
		return
	}

	Success(c, http.StatusOK, "OK", pos)
}
