package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServerPort  int
	DatabaseURL string
	DBHost      string
	DBPort      int
	DBName      string
	DBUser      string
	DBPassword  string
	JWTSecret   string
	GinMode     string
	LogLevel    string
	SecretsPath string
	BackupPath  string

	WeChatMerchantKey   string
	BackupEncryptionKey string
}

func Load() (*Config, error) {
	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	serverPort, err := strconv.Atoi(getEnv("SERVER_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid SERVER_PORT: %w", err)
	}

	cfg := &Config{
		ServerPort:  serverPort,
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      dbPort,
		DBName:      getEnv("DB_NAME", "campusrec"),
		DBUser:      getEnv("DB_USER", "campusrec"),
		DBPassword:  os.Getenv("DB_PASSWORD"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		GinMode:     getEnv("GIN_MODE", "release"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		SecretsPath: getEnv("SECRETS_PATH", "/run/secrets"),
		BackupPath:  getEnv("BACKUP_PATH", "/backups"),

		WeChatMerchantKey:   os.Getenv("WECHAT_MERCHANT_KEY"),
		BackupEncryptionKey: os.Getenv("BACKUP_ENCRYPTION_KEY"),
	}

	if cfg.DBPassword == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	cfg.DatabaseURL = fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
