package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	configService *services.ConfigService
}

func NewConfigHandler(configService *services.ConfigService) *ConfigHandler {
	return &ConfigHandler{configService: configService}
}

func (h *ConfigHandler) List(c *gin.Context) {
	entries, err := h.configService.ListConfig()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", entries)
}

type updateConfigRequest struct {
	Value            string `json:"value"`
	CanaryPercentage *int   `json:"canary_percentage"`
}

func (h *ConfigHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	ipAddress := c.ClientIP()

	entry, code, msg := h.configService.UpdateConfig(key, req.Value, req.CanaryPercentage, userID, ipAddress)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Configuration updated", entry)
}

func (h *ConfigHandler) ListCanary(c *gin.Context) {
	entries, err := h.configService.ListCanary()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", entries)
}

func (h *ConfigHandler) ListAuditLogs(c *gin.Context) {
	logs, err := h.configService.ListAuditLogs(50)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", logs)
}
