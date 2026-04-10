package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"campusrec/internal/config"
	"campusrec/internal/database"
	"campusrec/internal/handlers"
	"campusrec/internal/middleware"
	"campusrec/internal/repository"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: server <migrate|bootstrap-admin|serve>")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch os.Args[1] {
	case "migrate":
		runMigrate(cfg)
	case "bootstrap-admin":
		runBootstrapAdmin(cfg)
	case "serve":
		runServe(cfg)
	default:
		log.Fatalf("Unknown command: %s. Use 'migrate', 'bootstrap-admin', or 'serve'", os.Args[1])
	}
}

func runMigrate(cfg *config.Config) {
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	migrationsDir := "./migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = filepath.Join(".", "internal", "database", "migrations")
	}

	if err := database.RunMigrations(db, migrationsDir); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
}

func runBootstrapAdmin(cfg *config.Config) {
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE role = 'admin')").Scan(&exists)
	if err != nil {
		log.Fatalf("Failed to check for existing admin: %v", err)
	}

	if exists {
		log.Println("Admin user already exists, skipping bootstrap")
		return
	}

	passwordFile := filepath.Join(cfg.SecretsPath, "admin_bootstrap_password")
	passwordBytes, err := os.ReadFile(passwordFile)
	if err != nil {
		log.Fatalf("Failed to read admin bootstrap password from %s: %v. "+
			"This file must be created by init-secrets.sh before the backend starts.", passwordFile, err)
	}

	password := string(passwordBytes)
	if password == "" {
		log.Fatal("Admin bootstrap password file is empty")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash admin password: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (username, password_hash, role, display_name, status, created_at, updated_at)
		VALUES ('admin', $1, 'admin', 'Administrator', 'active', NOW(), NOW())
	`, string(hashedPassword))
	if err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	log.Println("Admin user bootstrapped. Read initial password from secrets volume: /run/secrets/admin_bootstrap_password")
}

func runServe(cfg *config.Config) {
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Repositories
	userRepo := repository.NewUserRepository(db)
	addressRepo := repository.NewAddressRepository(db)
	facilityRepo := repository.NewFacilityRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	productRepo := repository.NewProductRepository(db)
	catalogRepo := repository.NewCatalogRepository(db)
	regRepo := repository.NewRegistrationRepository(db)
	waitlistRepo := repository.NewWaitlistRepository(db)
	cartRepo := repository.NewCartRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	checkinRepo := repository.NewCheckInRepository(db)
	shippingRepo := repository.NewShippingRepository(db)
	postRepo := repository.NewPostRepository(db)
	ticketRepo := repository.NewTicketRepository(db)
	kpiRepo := repository.NewKPIRepository(db)
	jobRepo := repository.NewJobRepository(db)
	configRepo := repository.NewConfigRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	backupRepo := repository.NewBackupRepository(db)

	// Services
	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	userService := services.NewUserService(userRepo)
	addressService := services.NewAddressService(addressRepo)
	facilityService := services.NewFacilityService(facilityRepo, cfg.JWTSecret)
	sessionService := services.NewSessionService(sessionRepo, facilityRepo, configRepo)
	productService := services.NewProductService(productRepo)
	catalogService := services.NewCatalogService(catalogRepo)
	regService := services.NewRegistrationService(regRepo, sessionRepo, userRepo)
	cartService := services.NewCartService(cartRepo, productRepo)
	orderService := services.NewOrderService(orderRepo, productRepo, addressRepo, cartRepo, userRepo, auditRepo)
	checkinService := services.NewCheckInService(checkinRepo, regRepo, sessionRepo, facilityRepo, cfg.JWTSecret)
	shippingService := services.NewShippingService(shippingRepo, orderRepo, auditRepo)
	postService := services.NewPostService(postRepo, userRepo)
	ticketService := services.NewTicketService(ticketRepo, userRepo, auditRepo)
	kpiService := services.NewKPIService(kpiRepo)
	ieService := services.NewImportExportService(jobRepo)
	configService := services.NewConfigService(configRepo, auditRepo)
	backupService := services.NewBackupService(backupRepo, services.BackupConfig{
		DBHost:              cfg.DBHost,
		DBPort:              cfg.DBPort,
		DBName:              cfg.DBName,
		DBUser:              cfg.DBUser,
		DBPassword:          cfg.DBPassword,
		BackupPath:          cfg.BackupPath,
		BackupEncryptionKey: cfg.BackupEncryptionKey,
		WALArchivePath:      cfg.WALArchivePath,
	})
	paymentService := services.NewPaymentService(orderRepo, auditRepo, cfg.WeChatMerchantKey)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService)
	adminUserHandler := handlers.NewAdminUserHandler(userService)
	addressHandler := handlers.NewAddressHandler(addressService)
	adminFacilityHandler := handlers.NewAdminFacilityHandler(facilityService)
	sessionHandler := handlers.NewSessionHandler(sessionService)
	adminSessionHandler := handlers.NewAdminSessionHandler(sessionService)
	productHandler := handlers.NewProductHandler(productService)
	catalogHandler := handlers.NewCatalogHandler(catalogService)
	regHandler := handlers.NewRegistrationHandler(regService)
	adminRegHandler := handlers.NewAdminRegistrationHandler(regService)
	waitlistHandler := handlers.NewWaitlistHandler(waitlistRepo)
	cartHandler := handlers.NewCartHandler(cartService)
	orderHandler := handlers.NewOrderHandler(orderService)
	adminOrderHandler := handlers.NewAdminOrderHandler(orderService)
	checkinHandler := handlers.NewCheckInHandler(checkinService)
	shippingHandler := handlers.NewShippingHandler(shippingService)
	postHandler := handlers.NewPostHandler(postService)
	moderationHandler := handlers.NewModerationHandler(postService)
	ticketHandler := handlers.NewTicketHandler(ticketService)
	kpiHandler := handlers.NewKPIHandler(kpiService)
	ieHandler := handlers.NewImportExportHandler(ieService)
	configHandler := handlers.NewConfigHandler(configService)
	backupHandler := handlers.NewBackupHandler(backupService)
	paymentHandler := handlers.NewPaymentHandler(paymentService)

	// Page handler (Templ components are compiled into Go code)
	pageHandler := handlers.NewPageHandler("")

	gin.SetMode(cfg.GinMode)
	router := gin.New()

	// Global middleware chain: Recovery → Logging → CORS → RateLimit
	router.Use(gin.Recovery())
	router.Use(requestLogger())
	router.Use(middleware.CORS(cfg.AllowedOrigins))
	router.Use(middleware.RateLimit())

	// HTML page routes — public
	router.GET("/", pageHandler.Index)
	router.GET("/login", pageHandler.Login)

	// HTML page routes — authenticated member pages
	pages := router.Group("/")
	pages.Use(middleware.PageAuthRequired(cfg.JWTSecret))
	pages.GET("/dashboard", pageHandler.Dashboard)
	pages.GET("/catalog", pageHandler.Catalog)
	pages.GET("/registrations", pageHandler.Registrations)
	pages.GET("/cart", pageHandler.Cart)
	pages.GET("/checkout", pageHandler.Checkout)
	pages.GET("/orders", pageHandler.Orders)
	pages.GET("/orders/:id", pageHandler.OrderDetail)
	pages.GET("/addresses", pageHandler.Addresses)
	pages.GET("/posts", pageHandler.Posts)
	pages.GET("/tickets", pageHandler.Tickets)
	pages.GET("/ticket/new", pageHandler.TicketNew)
	pages.GET("/ticket/:id", pageHandler.TicketDetail)
	pages.GET("/sessions/:id", pageHandler.SessionDetail)
	pages.GET("/products/:id", pageHandler.ProductDetail)

	// HTML page routes — staff pages
	staffPages := router.Group("/")
	staffPages.Use(middleware.PageAuthRequired(cfg.JWTSecret))
	staffPages.Use(middleware.PageRequireRole("staff", "admin"))
	staffPages.GET("/checkin/status", pageHandler.CheckinStatus)
	staffPages.GET("/staff/shipping", pageHandler.StaffShipping)

	// HTML page routes — moderator pages
	modPages := router.Group("/")
	modPages.Use(middleware.PageAuthRequired(cfg.JWTSecret))
	modPages.Use(middleware.PageRequireRole("moderator", "admin"))
	modPages.GET("/moderation", pageHandler.Moderation)

	// HTML page routes — admin pages
	adminPages := router.Group("/admin")
	adminPages.Use(middleware.PageAuthRequired(cfg.JWTSecret))
	adminPages.Use(middleware.PageRequireRole("admin"))
	adminPages.GET("/users", pageHandler.AdminUsers)
	adminPages.GET("/sessions", pageHandler.AdminSessions)
	adminPages.GET("/kpi", pageHandler.AdminKPI)
	adminPages.GET("/config", pageHandler.AdminConfig)
	adminPages.GET("/import-export", pageHandler.AdminImportExport)
	adminPages.GET("/backup", pageHandler.AdminBackup)
	adminPages.GET("/tickets", pageHandler.AdminTickets)

	// Public endpoints
	router.GET("/api/health", healthHandler(db))

	api := router.Group("/api")

	// Auth endpoints (public)
	auth := api.Group("/auth")
	auth.POST("/login", authHandler.Login)

	// Auth endpoints (authenticated)
	authProtected := api.Group("/auth")
	authProtected.Use(middleware.AuthRequired(cfg.JWTSecret))
	authProtected.POST("/logout", authHandler.Logout)
	authProtected.GET("/me", authHandler.Me)
	authProtected.POST("/change-password", authHandler.ChangePassword)

	// Address endpoints (member only)
	addresses := api.Group("/addresses")
	addresses.Use(middleware.AuthRequired(cfg.JWTSecret))
	addresses.GET("", addressHandler.List)
	addresses.POST("", addressHandler.Create)
	addresses.PUT("/:id", addressHandler.Update)
	addresses.DELETE("/:id", addressHandler.Delete)
	addresses.PUT("/:id/default", addressHandler.SetDefault)

	// Session endpoints (authenticated)
	sessions := api.Group("/sessions")
	sessions.Use(middleware.AuthRequired(cfg.JWTSecret))
	sessions.GET("", sessionHandler.List)
	sessions.GET("/:id", sessionHandler.Get)

	// Product endpoints (authenticated)
	products := api.Group("/products")
	products.Use(middleware.AuthRequired(cfg.JWTSecret))
	products.GET("", productHandler.List)
	products.GET("/:id", productHandler.Get)

	// Catalog endpoint (authenticated)
	api.GET("/catalog", middleware.AuthRequired(cfg.JWTSecret), catalogHandler.Query)

	// Cart endpoints (member)
	cart := api.Group("/cart")
	cart.Use(middleware.AuthRequired(cfg.JWTSecret))
	cart.GET("", cartHandler.Get)
	cart.POST("", cartHandler.Add)
	cart.PUT("/:id", cartHandler.Update)
	cart.DELETE("/:id", cartHandler.Delete)

	// Registration endpoints (member)
	registrations := api.Group("/registrations")
	registrations.Use(middleware.AuthRequired(cfg.JWTSecret))
	registrations.POST("", regHandler.Create)
	registrations.GET("", regHandler.List)
	registrations.PUT("/:id/confirm", regHandler.Confirm)
	registrations.PUT("/:id/cancel", regHandler.Cancel)

	// Order endpoints (authenticated)
	orders := api.Group("/orders")
	orders.Use(middleware.AuthRequired(cfg.JWTSecret))
	orders.POST("", orderHandler.Create)
	orders.GET("", orderHandler.List)
	orders.GET("/:id", orderHandler.Get)
	orders.PUT("/:id/cancel", orderHandler.Cancel)
	orders.POST("/:id/complete", shippingHandler.CompleteOrder)

	// Payment endpoints
	api.POST("/payments/callback", paymentHandler.Callback)
	payments := api.Group("/payments")
	payments.Use(middleware.AuthRequired(cfg.JWTSecret))
	payments.Use(middleware.RequireRole("staff", "admin"))
	payments.POST("/:id/simulate-callback", paymentHandler.SimulateCallback)

	// Post endpoints (authenticated)
	posts := api.Group("/posts")
	posts.Use(middleware.AuthRequired(cfg.JWTSecret))
	posts.POST("", postHandler.Create)
	posts.GET("", postHandler.List)
	posts.POST("/:id/report", postHandler.Report)

	// Moderation endpoints (moderator/admin)
	moderation := api.Group("/moderation")
	moderation.Use(middleware.AuthRequired(cfg.JWTSecret))
	moderation.Use(middleware.RequireRole("moderator", "admin"))
	moderation.GET("/posts", moderationHandler.ListQueue)
	moderation.POST("/posts/:id/decision", moderationHandler.MakeDecision)

	// Ticket endpoints (authenticated — members see own, staff/admin see all)
	tickets := api.Group("/tickets")
	tickets.Use(middleware.AuthRequired(cfg.JWTSecret))
	tickets.POST("", ticketHandler.Create)
	tickets.GET("", ticketHandler.List)
	tickets.GET("/:id", ticketHandler.Get)
	tickets.PUT("/:id/assign", middleware.RequireRole("staff", "moderator", "admin"), ticketHandler.Assign)
	tickets.PUT("/:id/status", middleware.RequireRole("staff", "moderator", "admin"), ticketHandler.UpdateStatus)
	tickets.POST("/:id/comments", ticketHandler.AddComment)

	// Staff shipping endpoints
	staffShipping := api.Group("/staff/shipping")
	staffShipping.Use(middleware.AuthRequired(cfg.JWTSecret))
	staffShipping.Use(middleware.RequireRole("staff", "admin"))
	staffShipping.GET("", shippingHandler.List)
	staffShipping.PUT("/:id/ship", shippingHandler.Ship)
	staffShipping.PUT("/:id/deliver", shippingHandler.Deliver)
	staffShipping.PUT("/:id/exception", shippingHandler.Exception)

	// Check-in endpoints (staff performs check-in, member/staff can break/return)
	checkin := api.Group("/checkin")
	checkin.Use(middleware.AuthRequired(cfg.JWTSecret))
	checkin.POST("", middleware.RequireRole("staff", "admin"), checkinHandler.CheckIn)
	checkin.GET("/:id", checkinHandler.Get)
	checkin.POST("/:id/break", checkinHandler.StartBreak)
	checkin.POST("/:id/return", checkinHandler.ReturnFromBreak)

	// Session QR code (staff/admin)
	sessions.GET("/:id/qr", middleware.RequireRole("staff", "admin"), checkinHandler.GenerateQR)

	// Waitlist endpoint (member)
	api.GET("/waitlist/position", middleware.AuthRequired(cfg.JWTSecret), waitlistHandler.GetPosition)

	// Admin endpoints
	admin := api.Group("/admin")
	admin.Use(middleware.AuthRequired(cfg.JWTSecret))
	admin.Use(middleware.RequireRole("admin"))
	admin.GET("/users", adminUserHandler.ListUsers)
	admin.POST("/users", adminUserHandler.CreateUser)
	admin.PUT("/users/:id/status", adminUserHandler.UpdateStatus)
	admin.GET("/facilities", adminFacilityHandler.List)
	admin.POST("/facilities", adminFacilityHandler.Create)
	admin.PUT("/facilities/:id", adminFacilityHandler.Update)
	admin.POST("/facilities/:id/rotate-kiosk-token", adminFacilityHandler.RotateKioskToken)
	admin.POST("/sessions", adminSessionHandler.Create)
	admin.PUT("/sessions/:id", adminSessionHandler.Update)
	admin.PUT("/sessions/:id/status", adminSessionHandler.UpdateStatus)
	admin.GET("/registrations", adminRegHandler.List)
	admin.PUT("/registrations/:id/approve", adminRegHandler.Approve)
	admin.PUT("/registrations/:id/reject", adminRegHandler.Reject)
	admin.POST("/orders/:id/refund", adminOrderHandler.Refund)
	admin.GET("/config", configHandler.List)
	admin.GET("/config-canary", configHandler.ListCanary)
	admin.GET("/config-audit-logs", configHandler.ListAuditLogs)
	admin.PUT("/config/:key", configHandler.Update)
	admin.POST("/backup", backupHandler.TriggerBackup)
	admin.GET("/backups", backupHandler.ListBackups)
	admin.GET("/backup/restore-targets", backupHandler.RestoreTargets)
	admin.POST("/backup/restore", backupHandler.Restore)
	admin.POST("/archive/run", backupHandler.RunArchive)
	admin.GET("/archive/status", backupHandler.ArchiveStatus)

	// Import/Export endpoints (staff/admin)
	api.POST("/import", middleware.AuthRequired(cfg.JWTSecret), middleware.RequireRole("staff", "admin"), ieHandler.Import)
	api.GET("/export", middleware.AuthRequired(cfg.JWTSecret), middleware.RequireRole("staff", "admin"), ieHandler.Export)
	api.GET("/jobs/:id", middleware.AuthRequired(cfg.JWTSecret), ieHandler.GetJob)

	// KPI endpoints (admin only)
	kpi := api.Group("/kpi")
	kpi.Use(middleware.AuthRequired(cfg.JWTSecret))
	kpi.Use(middleware.RequireRole("admin"))
	kpi.GET("/overview", kpiHandler.Overview)
	kpi.GET("/fill-rate", kpiHandler.FillRate)
	kpi.GET("/members", kpiHandler.Members)
	kpi.GET("/engagement", kpiHandler.Engagement)
	kpi.GET("/coaches", kpiHandler.Coaches)
	kpi.GET("/revenue", kpiHandler.Revenue)
	kpi.GET("/tickets", kpiHandler.Tickets)

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("Starting server on %s", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func healthHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbStatus := "connected"
		if err := db.Ping(); err != nil {
			dbStatus = "disconnected"
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code": 503,
				"msg":  "Service unavailable",
				"data": gin.H{
					"status":   "unhealthy",
					"database": dbStatus,
					"version":  "1.0.0",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "OK",
			"data": gin.H{
				"status":   "healthy",
				"database": dbStatus,
				"version":  "1.0.0",
			},
		})
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Printf("%s %s %d %v", c.Request.Method, path, status, latency)
	}
}
