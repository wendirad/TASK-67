package handlers

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Success sends a standard success response.
func Success(c *gin.Context, code int, msg string, data interface{}) {
	body := gin.H{"code": code, "msg": msg}
	if data != nil {
		body["data"] = data
	}
	c.JSON(code, body)
}

// Error sends a standard error response.
func Error(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"code": code, "msg": msg})
}

// PaginationParams holds parsed pagination query params.
type PaginationParams struct {
	Page     int
	PageSize int
	Offset   int
}

// ParsePagination extracts page and page_size from query params with defaults and limits.
func ParsePagination(c *gin.Context) PaginationParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return PaginationParams{
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
	}
}

// PaginatedResponse builds the standard paginated response body.
func PaginatedResponse(items interface{}, total int, p PaginationParams) gin.H {
	totalPages := int(math.Ceil(float64(total) / float64(p.PageSize)))
	return gin.H{
		"items":       items,
		"total":       total,
		"page":        p.Page,
		"page_size":   p.PageSize,
		"total_pages": totalPages,
	}
}

// Created sends a 201 response.
func Created(c *gin.Context, msg string, data interface{}) {
	Success(c, http.StatusCreated, msg, data)
}
