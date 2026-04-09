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

	gin.SetMode(cfg.GinMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger())

	router.GET("/api/health", healthHandler(db))

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
