package handlers

import (
	"net/http"

	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	sessionService *services.SessionService
}

func NewSessionHandler(sessionService *services.SessionService) *SessionHandler {
	return &SessionHandler{sessionService: sessionService}
}

func (h *SessionHandler) List(c *gin.Context) {
	p := ParsePagination(c)
	status := c.Query("status")
	facility := c.Query("facility")
	search := c.Query("search")
	fromDate := c.Query("from_date")
	toDate := c.Query("to_date")

	sessions, total, err := h.sessionService.ListSessions(p.Page, p.PageSize, status, facility, search, fromDate, toDate)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if sessions == nil {
		sessions = []models.Session{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(sessions, total, p))
}

func (h *SessionHandler) Get(c *gin.Context) {
	id := c.Param("id")
	session, err := h.sessionService.GetSession(id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if session == nil {
		Error(c, http.StatusNotFound, "Session not found")
		return
	}
	Success(c, http.StatusOK, "OK", session)
}
