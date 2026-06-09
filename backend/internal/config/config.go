package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv            string
	HTTPAddr          string
	DatabaseURL       string
	MigrationsDir     string
	RunMigrations     bool
	AdminUsername     string
	AdminPassword     string
	EncryptionKey     string
	APIAuthRequired   bool
	CorsOrigins       []string
	UpstreamUserAgent string
	RequestTimeout    time.Duration
}

func Load() Config {
	_ = godotenv.Load()
	return Config{
		AppEnv:            getString("APP_ENV", "development"),
		HTTPAddr:          getString("HTTP_ADDR", ":8080"),
		DatabaseURL:       getString("DATABASE_URL", "postgres://one_search:one_search@localhost:5432/one_search?sslmode=disable"),
		MigrationsDir:     getString("MIGRATIONS_DIR", "migrations"),
		RunMigrations:     getBool("RUN_MIGRATIONS", true),
		AdminUsername:     getString("ADMIN_USERNAME", "admin"),
		AdminPassword:     getString("ADMIN_PASSWORD", "admin123456"),
		EncryptionKey:     getString("ENCRYPTION_KEY", "change-me-32-byte-encryption-key"),
		APIAuthRequired:   getBool("API_AUTH_REQUIRED", true),
		CorsOrigins:       getCSV("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:8080"),
		UpstreamUserAgent: getString("UPSTREAM_USER_AGENT", "OneSearchRelay/0.1"),
		RequestTimeout:    time.Duration(getInt("REQUEST_TIMEOUT_MS", 20000)) * time.Millisecond,
	}
}

func getString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getCSV(key, fallback string) []string {
	value := getString(key, fallback)
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
