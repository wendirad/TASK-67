package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type CartHandler struct {
	cartService *services.CartService
}

func NewCartHandler(cartService *services.CartService) *CartHandler {
	return &CartHandler{cartService: cartService}
}

func (h *CartHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	cart, err := h.cartService.GetCart(userID)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	Success(c, http.StatusOK, "OK", cart)
}

type addToCartRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required"`
}

func (h *CartHandler) Add(c *gin.Context) {
	var req addToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "product_id and quantity are required")
		return
	}

	userID := middleware.GetUserID(c)
	code, msg := h.cartService.AddToCart(userID, req.ProductID, req.Quantity)
	if code != http.StatusCreated {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusCreated, "Added to cart", nil)
}

type updateCartRequest struct {
	Quantity int `json:"quantity" binding:"required"`
}

func (h *CartHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "quantity is required")
		return
	}

	userID := middleware.GetUserID(c)
	code, msg := h.cartService.UpdateQuantity(userID, id, req.Quantity)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Cart updated", nil)
}

func (h *CartHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	code, msg := h.cartService.RemoveItem(userID, id)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Item removed", nil)
}
