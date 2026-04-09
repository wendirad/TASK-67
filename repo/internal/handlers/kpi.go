package handlers

import (
	"net/http"
	"time"

	"campusrec/internal/repository"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type KPIHandler struct {
	kpiService *services.KPIService
}

func NewKPIHandler(kpiService *services.KPIService) *KPIHandler {
	return &KPIHandler{kpiService: kpiService}
}

// parseDateRange extracts from_date and to_date query params with defaults.
func parseDateRange(c *gin.Context) (time.Time, time.Time) {
	now := time.Now()
	fromStr := c.Query("from_date")
	toStr := c.Query("to_date")

	from := now.AddDate(0, -1, 0) // default: last 30 days
	to := now

	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			// Set to end of day
			to = t.Add(24*time.Hour - time.Second)
		}
	}

	return from, to
}

func (h *KPIHandler) Overview(c *gin.Context) {
	from, to := parseDateRange(c)
	facility := c.Query("facility")

	result, err := h.kpiService.Overview(from, to, facility)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", result)
}

func (h *KPIHandler) FillRate(c *gin.Context) {
	from, to := parseDateRange(c)
	facility := c.Query("facility")
	granularity := c.DefaultQuery("granularity", "daily")

	result, err := h.kpiService.FillRate(from, to, facility, granularity)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if result == nil {
		result = []repository.FillRateTimePoint{}
	}

	Success(c, http.StatusOK, "OK", result)
}

func (h *KPIHandler) Members(c *gin.Context) {
	from, to := parseDateRange(c)
	granularity := c.DefaultQuery("granularity", "weekly")

	result, err := h.kpiService.Members(from, to, granularity)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if result == nil {
		result = []repository.MemberTimePoint{}
	}

	Success(c, http.StatusOK, "OK", result)
}

func (h *KPIHandler) Engagement(c *gin.Context) {
	from, to := parseDateRange(c)

	result, err := h.kpiService.Engagement(from, to)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", result)
}

func (h *KPIHandler) Coaches(c *gin.Context) {
	from, to := parseDateRange(c)

	result, err := h.kpiService.Coaches(from, to)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if result == nil {
		result = []repository.CoachResult{}
	}

	Success(c, http.StatusOK, "OK", result)
}

func (h *KPIHandler) Revenue(c *gin.Context) {
	from, to := parseDateRange(c)
	granularity := c.DefaultQuery("granularity", "daily")

	result, err := h.kpiService.Revenue(from, to, granularity)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", result)
}

func (h *KPIHandler) Tickets(c *gin.Context) {
	result, err := h.kpiService.Tickets()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	Success(c, http.StatusOK, "OK", result)
}
