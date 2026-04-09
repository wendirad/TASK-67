package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// PageHandler serves HTML pages using Go templates.
type PageHandler struct {
	templates map[string]*template.Template
}

// NewPageHandler loads all page templates with the base layout.
func NewPageHandler(templatesDir string) *PageHandler {
	h := &PageHandler{
		templates: make(map[string]*template.Template),
	}

	layoutFile := filepath.Join(templatesDir, "layouts", "base.html")

	pages := []string{
		"login", "dashboard", "catalog", "registrations", "cart", "checkout",
		"orders", "order_detail", "addresses", "posts", "tickets",
		"ticket_detail", "ticket_new", "session_detail", "product_detail",
		"checkin_status", "staff_shipping", "moderation",
		"admin_users", "admin_sessions", "admin_kpi", "admin_config",
		"admin_import_export", "admin_backup", "admin_tickets",
	}

	for _, page := range pages {
		pageFile := filepath.Join(templatesDir, "pages", page+".html")
		tmpl, err := template.ParseFiles(layoutFile, pageFile)
		if err != nil {
			log.Fatalf("Failed to parse template %s: %v", page, err)
		}
		h.templates[page] = tmpl
	}

	return h
}

type pageData struct {
	Title         string
	Authenticated bool
}

func (h *PageHandler) render(c *gin.Context, page string, data pageData) {
	tmpl, ok := h.templates[page]
	if !ok {
		c.String(http.StatusNotFound, "Page not found")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		log.Printf("Template render error for %s: %v", page, err)
	}
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
	h.render(c, "login", pageData{Title: "Login"})
}

func (h *PageHandler) Index(c *gin.Context) {
	if h.isAuthenticated(c) {
		c.Redirect(http.StatusFound, "/dashboard")
	} else {
		c.Redirect(http.StatusFound, "/login")
	}
}

// Authenticated pages
func (h *PageHandler) Dashboard(c *gin.Context)       { h.render(c, "dashboard", pageData{Title: "Dashboard", Authenticated: true}) }
func (h *PageHandler) Catalog(c *gin.Context)          { h.render(c, "catalog", pageData{Title: "Catalog", Authenticated: true}) }
func (h *PageHandler) Registrations(c *gin.Context)    { h.render(c, "registrations", pageData{Title: "Registrations", Authenticated: true}) }
func (h *PageHandler) Cart(c *gin.Context)             { h.render(c, "cart", pageData{Title: "Cart", Authenticated: true}) }
func (h *PageHandler) Checkout(c *gin.Context)         { h.render(c, "checkout", pageData{Title: "Checkout", Authenticated: true}) }
func (h *PageHandler) Orders(c *gin.Context)           { h.render(c, "orders", pageData{Title: "Orders", Authenticated: true}) }
func (h *PageHandler) OrderDetail(c *gin.Context)      { h.render(c, "order_detail", pageData{Title: "Order", Authenticated: true}) }
func (h *PageHandler) Addresses(c *gin.Context)        { h.render(c, "addresses", pageData{Title: "Addresses", Authenticated: true}) }
func (h *PageHandler) Posts(c *gin.Context)            { h.render(c, "posts", pageData{Title: "Posts", Authenticated: true}) }
func (h *PageHandler) Tickets(c *gin.Context)          { h.render(c, "tickets", pageData{Title: "Tickets", Authenticated: true}) }
func (h *PageHandler) TicketDetail(c *gin.Context)     { h.render(c, "ticket_detail", pageData{Title: "Ticket", Authenticated: true}) }
func (h *PageHandler) TicketNew(c *gin.Context)        { h.render(c, "ticket_new", pageData{Title: "New Ticket", Authenticated: true}) }
func (h *PageHandler) SessionDetail(c *gin.Context)    { h.render(c, "session_detail", pageData{Title: "Session", Authenticated: true}) }
func (h *PageHandler) ProductDetail(c *gin.Context)    { h.render(c, "product_detail", pageData{Title: "Product", Authenticated: true}) }
func (h *PageHandler) CheckinStatus(c *gin.Context)    { h.render(c, "checkin_status", pageData{Title: "Check-in", Authenticated: true}) }
func (h *PageHandler) StaffShipping(c *gin.Context)    { h.render(c, "staff_shipping", pageData{Title: "Shipping", Authenticated: true}) }
func (h *PageHandler) Moderation(c *gin.Context)       { h.render(c, "moderation", pageData{Title: "Moderation", Authenticated: true}) }
func (h *PageHandler) AdminUsers(c *gin.Context)       { h.render(c, "admin_users", pageData{Title: "Users", Authenticated: true}) }
func (h *PageHandler) AdminSessions(c *gin.Context)    { h.render(c, "admin_sessions", pageData{Title: "Sessions", Authenticated: true}) }
func (h *PageHandler) AdminKPI(c *gin.Context)         { h.render(c, "admin_kpi", pageData{Title: "KPI", Authenticated: true}) }
func (h *PageHandler) AdminConfig(c *gin.Context)      { h.render(c, "admin_config", pageData{Title: "Config", Authenticated: true}) }
func (h *PageHandler) AdminImportExport(c *gin.Context) { h.render(c, "admin_import_export", pageData{Title: "Import/Export", Authenticated: true}) }
func (h *PageHandler) AdminBackup(c *gin.Context)      { h.render(c, "admin_backup", pageData{Title: "Backup", Authenticated: true}) }
func (h *PageHandler) AdminTickets(c *gin.Context)     { h.render(c, "admin_tickets", pageData{Title: "Tickets", Authenticated: true}) }
