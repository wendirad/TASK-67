package handlers

import (
	"net/http"
	"strconv"

	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type ProductHandler struct {
	productService *services.ProductService
}

func NewProductHandler(productService *services.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

func (h *ProductHandler) List(c *gin.Context) {
	p := ParsePagination(c)
	category := c.Query("category")
	search := c.Query("search")
	status := c.Query("status")

	var minPrice, maxPrice *int
	if v := c.Query("min_price"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			minPrice = &n
		}
	}
	if v := c.Query("max_price"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxPrice = &n
		}
	}

	var isShippable *bool
	if v := c.Query("is_shippable"); v != "" {
		b := v == "true"
		isShippable = &b
	}

	products, total, err := h.productService.ListProducts(p.Page, p.PageSize, category, search, status, minPrice, maxPrice, isShippable)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if products == nil {
		products = []models.Product{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(products, total, p))
}

func (h *ProductHandler) Get(c *gin.Context) {
	id := c.Param("id")
	product, err := h.productService.GetProduct(id)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if product == nil {
		Error(c, http.StatusNotFound, "Product not found")
		return
	}
	Success(c, http.StatusOK, "OK", product)
}
