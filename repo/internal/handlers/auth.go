package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService  *services.AuthService
	cookieSecure bool
}

func NewAuthHandler(authService *services.AuthService, cookieSecure bool) *AuthHandler {
	return &AuthHandler{authService: authService, cookieSecure: cookieSecure}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		Error(c, http.StatusBadRequest, "Username and password are required")
		return
	}

	result, code, msg := h.authService.Login(req.Username, req.Password)
	if result == nil {
		Error(c, code, msg)
		return
	}

	c.SetCookie("session_token", result.Token, 86400, "/", "", h.cookieSecure, true)

	Success(c, http.StatusOK, "Login successful", gin.H{
		"token": result.Token,
		"user": gin.H{
			"id":           result.User.ID,
			"username":     result.User.Username,
			"role":         result.User.Role,
			"display_name": result.User.DisplayName,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("session_token", "", -1, "/", "", h.cookieSecure, true)
	Success(c, http.StatusOK, "Logged out successfully", nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.authService.GetUserProfile(userID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if user == nil {
		Error(c, http.StatusNotFound, "User not found")
		return
	}

	Success(c, http.StatusOK, "OK", gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"role":         user.Role,
		"display_name": user.DisplayName,
		"email":        user.Email,
		"phone":        user.Phone,
		"status":       user.Status,
		"created_at":   user.CreatedAt,
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		Error(c, http.StatusBadRequest, "Current password and new password are required")
		return
	}

	if ok, msg := ValidatePassword(req.NewPassword); !ok {
		Error(c, http.StatusBadRequest, msg)
		return
	}

	userID := middleware.GetUserID(c)
	code, msg := h.authService.ChangePassword(userID, req.CurrentPassword, req.NewPassword)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Password changed successfully", nil)
}
