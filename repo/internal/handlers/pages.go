package handlers

import (
	"net/http"

	"campusrec/internal/templates"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

// PageHandler serves HTML pages using Templ components.
type PageHandler struct{}

// NewPageHandler creates a new page handler (no template directory needed with Templ).
func NewPageHandler(_ string) *PageHandler {
	return &PageHandler{}
}

func render(c *gin.Context, component templ.Component) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	component.Render(c.Request.Context(), c.Writer)
}

func (h *PageHandler) isAuthenticated(c *gin.Context) bool {
	cookie, err := c.Cookie("session_token")
	return err == nil && cookie != ""
}

// Public pages
func (h *PageHandler) Login(c *gin.Context) {
	if h.isAuthenticated(c) {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	render(c, templates.LoginPage())
}

func (h *PageHandler) Index(c *gin.Context) {
	if h.isAuthenticated(c) {
		c.Redirect(http.StatusFound, "/dashboard")
	} else {
		c.Redirect(http.StatusFound, "/login")
	}
}

// Authenticated pages
func (h *PageHandler) Dashboard(c *gin.Context)         { render(c, templates.DashboardPage()) }
func (h *PageHandler) Catalog(c *gin.Context)            { render(c, templates.CatalogPage()) }
func (h *PageHandler) Registrations(c *gin.Context)      { render(c, templates.RegistrationsPage()) }
func (h *PageHandler) Cart(c *gin.Context)               { render(c, templates.CartPage()) }
func (h *PageHandler) Checkout(c *gin.Context)           { render(c, templates.CheckoutPage()) }
func (h *PageHandler) Orders(c *gin.Context)             { render(c, templates.OrdersPage()) }
func (h *PageHandler) OrderDetail(c *gin.Context)        { render(c, templates.OrderDetailPage()) }
func (h *PageHandler) Addresses(c *gin.Context)          { render(c, templates.AddressesPage()) }
func (h *PageHandler) Posts(c *gin.Context)              { render(c, templates.PostsPage()) }
func (h *PageHandler) Tickets(c *gin.Context)            { render(c, templates.TicketsPage()) }
func (h *PageHandler) TicketDetail(c *gin.Context)       { render(c, templates.TicketDetailPage()) }
func (h *PageHandler) TicketNew(c *gin.Context)          { render(c, templates.TicketNewPage()) }
func (h *PageHandler) SessionDetail(c *gin.Context)      { render(c, templates.SessionDetailPage()) }
func (h *PageHandler) ProductDetail(c *gin.Context)      { render(c, templates.ProductDetailPage()) }
func (h *PageHandler) CheckinStatus(c *gin.Context)      { render(c, templates.CheckinStatusPage()) }
func (h *PageHandler) StaffShipping(c *gin.Context)      { render(c, templates.StaffShippingPage()) }
func (h *PageHandler) Moderation(c *gin.Context)         { render(c, templates.ModerationPage()) }
func (h *PageHandler) AdminUsers(c *gin.Context)         { render(c, templates.AdminUsersPage()) }
func (h *PageHandler) AdminSessions(c *gin.Context)      { render(c, templates.AdminSessionsPage()) }
func (h *PageHandler) AdminKPI(c *gin.Context)           { render(c, templates.AdminKPIPage()) }
func (h *PageHandler) AdminConfig(c *gin.Context)        { render(c, templates.AdminConfigPage()) }
func (h *PageHandler) AdminImportExport(c *gin.Context)  { render(c, templates.AdminImportExportPage()) }
func (h *PageHandler) AdminBackup(c *gin.Context)        { render(c, templates.AdminBackupPage()) }
func (h *PageHandler) AdminTickets(c *gin.Context)       { render(c, templates.AdminTicketsPage()) }
