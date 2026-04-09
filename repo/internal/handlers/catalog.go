package handlers

import (
	"net/http"

	"campusrec/internal/models"
	"campusrec/internal/repository"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type CatalogHandler struct {
	catalogService *services.CatalogService
}

func NewCatalogHandler(catalogService *services.CatalogService) *CatalogHandler {
	return &CatalogHandler{catalogService: catalogService}
}

func (h *CatalogHandler) Query(c *gin.Context) {
	p := ParsePagination(c)

	typeFilter := c.DefaultQuery("type", "all")
	if typeFilter != "all" && typeFilter != "session" && typeFilter != "product" {
		Error(c, http.StatusBadRequest, "Invalid type filter. Must be: all, session, or product")
		return
	}

	sort := c.DefaultQuery("sort", "relevance")
	validSorts := map[string]bool{
		"relevance": true, "price_asc": true, "price_desc": true,
		"date_asc": true, "date_desc": true, "name_asc": true,
	}
	if !validSorts[sort] {
		Error(c, http.StatusBadRequest, "Invalid sort. Must be: relevance, price_asc, price_desc, date_asc, date_desc, or name_asc")
		return
	}

	q := repository.CatalogQuery{
		Page:     p.Page,
		PageSize: p.PageSize,
		Type:     typeFilter,
		Search:   c.Query("search"),
		Category: c.Query("category"),
		Facility: c.Query("facility"),
		FromDate: c.Query("from_date"),
		ToDate:   c.Query("to_date"),
		Sort:     sort,
	}

	items, total, err := h.catalogService.Query(q)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if items == nil {
		items = []models.CatalogItem{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(items, total, p))
}
