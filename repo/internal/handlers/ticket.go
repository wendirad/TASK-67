package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type TicketHandler struct {
	ticketService *services.TicketService
}

func NewTicketHandler(ticketService *services.TicketService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService}
}

type createTicketRequest struct {
	Type              string  `json:"type"`
	Subject           string  `json:"subject"`
	Description       string  `json:"description"`
	Priority          string  `json:"priority"`
	RelatedEntityType *string `json:"related_entity_type"`
	RelatedEntityID   *string `json:"related_entity_id"`
}

func (h *TicketHandler) Create(c *gin.Context) {
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	ticket, code, msg := h.ticketService.CreateTicket(
		userID, req.Type, req.Subject, req.Description, req.Priority,
		req.RelatedEntityType, req.RelatedEntityID,
	)
	if code != http.StatusCreated {
		Error(c, code, msg)
		return
	}

	Created(c, "Ticket created", ticket)
}

func (h *TicketHandler) List(c *gin.Context) {
	p := ParsePagination(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	status := c.Query("status")
	ticketType := c.Query("type")
	priority := c.Query("priority")
	assignedTo := c.Query("assigned_to")

	tickets, total, err := h.ticketService.ListTickets(userID, role, p.Page, p.PageSize, status, ticketType, priority, assignedTo)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if tickets == nil {
		tickets = []models.Ticket{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(tickets, total, p))
}

func (h *TicketHandler) Get(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	ticket, code, msg := h.ticketService.GetTicket(id, userID, role)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "OK", ticket)
}

type assignTicketRequest struct {
	AssignedTo string `json:"assigned_to"`
}

func (h *TicketHandler) Assign(c *gin.Context) {
	id := c.Param("id")
	var req assignTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.AssignedTo == "" {
		Error(c, http.StatusBadRequest, "assigned_to is required")
		return
	}

	performedBy := middleware.GetUserID(c)
	code, msg := h.ticketService.AssignTicket(id, req.AssignedTo, performedBy)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Ticket assigned", nil)
}

type updateTicketStatusRequest struct {
	Status string `json:"status"`
}

func (h *TicketHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req updateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	code, msg := h.ticketService.UpdateTicketStatus(id, userID, role, req.Status)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Ticket status updated", nil)
}

type addCommentRequest struct {
	Content string `json:"content"`
}

func (h *TicketHandler) AddComment(c *gin.Context) {
	id := c.Param("id")
	var req addCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	comment, code, msg := h.ticketService.AddComment(id, userID, role, req.Content)
	if code != http.StatusCreated {
		Error(c, code, msg)
		return
	}

	Created(c, "Comment added", comment)
}
