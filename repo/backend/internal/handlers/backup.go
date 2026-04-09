package handlers

import (
	"net/http"
	"time"

	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type BackupHandler struct {
	backupService *services.BackupService
}

func NewBackupHandler(backupService *services.BackupService) *BackupHandler {
	return &BackupHandler{backupService: backupService}
}

func (h *BackupHandler) TriggerBackup(c *gin.Context) {
	backup, code, msg := h.backupService.TriggerBackup()
	if code != 202 {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusAccepted, msg, backup)
}

func (h *BackupHandler) ListBackups(c *gin.Context) {
	backups, err := h.backupService.ListBackups()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", backups)
}

func (h *BackupHandler) RestoreTargets(c *gin.Context) {
	targets, err := h.backupService.GetRestoreTargets()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", targets)
}

type restoreRequest struct {
	RestoreType       string  `json:"restore_type"`
	BackupID          string  `json:"backup_id"`
	TargetTime        *string `json:"target_time"`
	ConfirmationToken string  `json:"confirmation_token"`
}

func (h *BackupHandler) Restore(c *gin.Context) {
	var req restoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	var targetTime *time.Time
	if req.TargetTime != nil && *req.TargetTime != "" {
		t, err := time.Parse(time.RFC3339, *req.TargetTime)
		if err != nil {
			Error(c, http.StatusBadRequest, "Invalid target_time format, use ISO8601/RFC3339")
			return
		}
		targetTime = &t
	}

	backup, code, msg := h.backupService.TriggerRestore(req.RestoreType, req.BackupID, req.ConfirmationToken, targetTime)
	if code != 202 {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusAccepted, msg, backup)
}

func (h *BackupHandler) RunArchive(c *gin.Context) {
	status, code, msg := h.backupService.RunArchive()
	if code != 202 {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusAccepted, msg, status)
}

func (h *BackupHandler) ArchiveStatus(c *gin.Context) {
	status, err := h.backupService.GetArchiveStatus()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", status)
}
