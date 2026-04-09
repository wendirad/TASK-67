package handlers

import (
	"net/http"

	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminUserHandler struct {
	userService *services.UserService
}

func NewAdminUserHandler(userService *services.UserService) *AdminUserHandler {
	return &AdminUserHandler{userService: userService}
}

var validRoles = map[string]bool{
	"member":    true,
	"staff":     true,
	"moderator": true,
	"admin":     true,
}

var validStatuses = map[string]bool{
	"active":    true,
	"banned":    true,
	"suspended": true,
	"inactive":  true,
}

func (h *AdminUserHandler) ListUsers(c *gin.Context) {
	p := ParsePagination(c)
	role := c.Query("role")
	status := c.Query("status")
	search := c.Query("search")

	if role != "" && !validRoles[role] {
		Error(c, http.StatusBadRequest, "Invalid role filter")
		return
	}
	if status != "" && !validStatuses[status] {
		Error(c, http.StatusBadRequest, "Invalid status filter")
		return
	}

	users, total, err := h.userService.ListUsers(p.Page, p.PageSize, role, status, search)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	if users == nil {
		users = []models.User{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(users, total, p))
}

func (h *AdminUserHandler) CreateUser(c *gin.Context) {
	var req services.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" {
		Error(c, http.StatusBadRequest, "Username is required")
		return
	}
	if req.Password == "" {
		Error(c, http.StatusBadRequest, "Password is required")
		return
	}
	if req.DisplayName == "" {
		Error(c, http.StatusBadRequest, "Display name is required")
		return
	}
	if !validRoles[req.Role] {
		Error(c, http.StatusBadRequest, "Invalid role. Must be one of: member, staff, moderator, admin")
		return
	}

	if ok, msg := ValidatePassword(req.Password); !ok {
		Error(c, http.StatusBadRequest, msg)
		return
	}

	user, code, msg := h.userService.CreateUser(req)
	if user == nil {
		Error(c, code, msg)
		return
	}

	Created(c, "User created successfully", user)
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

func (h *AdminUserHandler) UpdateStatus(c *gin.Context) {
	userID := c.Param("id")

	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !validStatuses[req.Status] {
		Error(c, http.StatusBadRequest, "Invalid status. Must be one of: active, banned, suspended, inactive")
		return
	}

	user, code, msg := h.userService.UpdateUserStatus(userID, req.Status)
	if user == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "User status updated", user)
}
