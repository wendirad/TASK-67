package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type ImportExportHandler struct {
	ieService *services.ImportExportService
}

func NewImportExportHandler(ieService *services.ImportExportService) *ImportExportHandler {
	return &ImportExportHandler{ieService: ieService}
}

func (h *ImportExportHandler) Import(c *gin.Context) {
	entityType := c.PostForm("entity_type")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		Error(c, http.StatusBadRequest, "File is required")
		return
	}
	defer file.Close()

	job, validationResult, code, msg := h.ieService.Import(
		middleware.GetUserID(c), entityType, file, header,
	)

	if code == http.StatusBadRequest && validationResult != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  msg,
			"data": validationResult,
		})
		return
	}

	if code != http.StatusAccepted {
		Error(c, code, msg)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"code": 202,
		"msg":  "Import job created",
		"data": gin.H{"job_id": job.ID},
	})
}

func (h *ImportExportHandler) Export(c *gin.Context) {
	entityType := c.Query("entity_type")
	format := c.DefaultQuery("format", "csv")
	filters := c.Query("filters")

	userID := middleware.GetUserID(c)
	job, code, msg := h.ieService.Export(userID, entityType, format, filters)
	if code != http.StatusAccepted {
		Error(c, code, msg)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"code": 202,
		"msg":  "Export job created",
		"data": gin.H{"job_id": job.ID},
	})
}

func (h *ImportExportHandler) GetJob(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	job, code, msg := h.ieService.GetJob(id, userID, userRole)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "OK", job)
}
