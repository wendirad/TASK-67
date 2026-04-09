package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	orderService *services.OrderService
}

func NewOrderHandler(orderService *services.OrderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

func (h *OrderHandler) Create(c *gin.Context) {
	var req models.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	order, code, msg := h.orderService.CreateOrder(userID, &req)
	if code != http.StatusCreated {
		Error(c, code, msg)
		return
	}

	Created(c, "Order created", order)
}

func (h *OrderHandler) List(c *gin.Context) {
	p := ParsePagination(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	status := c.Query("status")

	orders, total, err := h.orderService.ListOrders(userID, role, p.Page, p.PageSize, status)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if orders == nil {
		orders = []models.Order{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(orders, total, p))
}

func (h *OrderHandler) Get(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	order, code, msg := h.orderService.GetOrder(id, userID, role)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "OK", order)
}

func (h *OrderHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	code, msg := h.orderService.CancelOrder(id, userID, role)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Order canceled", nil)
}

// AdminOrderHandler handles admin-only order operations.
type AdminOrderHandler struct {
	orderService *services.OrderService
}

func NewAdminOrderHandler(orderService *services.OrderService) *AdminOrderHandler {
	return &AdminOrderHandler{orderService: orderService}
}

func (h *AdminOrderHandler) Refund(c *gin.Context) {
	id := c.Param("id")

	code, msg := h.orderService.RefundOrder(id)
	if code != http.StatusOK {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Refund processed", nil)
}
