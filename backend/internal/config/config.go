package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv                  string
	HTTPAddr                string
	DatabaseURL             string
	MigrationsDir           string
	RunMigrations           bool
	AdminUsername           string
	AdminPassword           string
	EncryptionKey           string
	APIAuthRequired         bool
	MCPEnabled              bool
	MCPPath                 string
	CorsOrigins             []string
	UpstreamUserAgent       string
	RequestTimeout          time.Duration
	RequestBodyLimitBytes   int64
	ServerReadHeaderTimeout time.Duration
	ServerReadTimeout       time.Duration
	ServerWriteTimeout      time.Duration
	ServerIdleTimeout       time.Duration
	AdminSessionTTL         time.Duration
	AdminLoginMaxAttempts   int
	AdminLoginWindow        time.Duration
	AdminLoginLockout       time.Duration
}

func Load() (Config, error) {
	loadEnvFiles()
	appEnv := getString("APP_ENV", "development")
	production := isProduction(appEnv)
	adminPasswordFallback := ""
	encryptionKeyFallback := ""
	if !production {
		adminPasswordFallback = "admin123456"
		encryptionKeyFallback = "development-only-change-me-32-byte-key"
	}
	cfg := Config{
		AppEnv:                  appEnv,
		HTTPAddr:                getString("HTTP_ADDR", ":8080"),
		DatabaseURL:             getString("DATABASE_URL", "postgres://one_search:one_search@localhost:5432/one_search?sslmode=disable"),
		MigrationsDir:           getString("MIGRATIONS_DIR", "migrations"),
		RunMigrations:           getBool("RUN_MIGRATIONS", true),
		AdminUsername:           getString("ADMIN_USERNAME", "admin"),
		AdminPassword:           getString("ADMIN_PASSWORD", adminPasswordFallback),
		EncryptionKey:           getString("ENCRYPTION_KEY", encryptionKeyFallback),
		APIAuthRequired:         getBool("API_AUTH_REQUIRED", true),
		MCPEnabled:              getBool("MCP_ENABLED", false),
		MCPPath:                 normalizePath(getString("MCP_PATH", "/mcp")),
		CorsOrigins:             getCSV("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:8080"),
		UpstreamUserAgent:       getString("UPSTREAM_USER_AGENT", "OneSearchRelay/0.1"),
		RequestTimeout:          time.Duration(getInt("REQUEST_TIMEOUT_MS", 20000)) * time.Millisecond,
		RequestBodyLimitBytes:   int64(getInt("REQUEST_BODY_LIMIT_BYTES", 1048576)),
		ServerReadHeaderTimeout: time.Duration(getInt("SERVER_READ_HEADER_TIMEOUT_MS", 10000)) * time.Millisecond,
		ServerReadTimeout:       time.Duration(getInt("SERVER_READ_TIMEOUT_MS", 30000)) * time.Millisecond,
		ServerWriteTimeout:      time.Duration(getInt("SERVER_WRITE_TIMEOUT_MS", 30000)) * time.Millisecond,
		ServerIdleTimeout:       time.Duration(getInt("SERVER_IDLE_TIMEOUT_MS", 60000)) * time.Millisecond,
		AdminSessionTTL:         time.Duration(getInt("ADMIN_SESSION_TTL_HOURS", 24)) * time.Hour,
		AdminLoginMaxAttempts:   getInt("ADMIN_LOGIN_MAX_ATTEMPTS", 5),
		AdminLoginWindow:        time.Duration(getInt("ADMIN_LOGIN_WINDOW_MS", 300000)) * time.Millisecond,
		AdminLoginLockout:       time.Duration(getInt("ADMIN_LOGIN_LOCKOUT_MS", 900000)) * time.Millisecond,
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.AdminUsername) == "" {
		return fmt.Errorf("ADMIN_USERNAME is required")
	}
	if c.AdminSessionTTL <= 0 {
		return fmt.Errorf("ADMIN_SESSION_TTL_HOURS must be positive")
	}
	if isProduction(c.AppEnv) {
		if strings.TrimSpace(c.EncryptionKey) == "" {
			return fmt.Errorf("ENCRYPTION_KEY is required in production")
		}
		if isKnownWeakSecret(c.EncryptionKey) || len(c.EncryptionKey) < 32 {
			return fmt.Errorf("ENCRYPTION_KEY must be a strong production secret with at least 32 characters")
		}
		if strings.TrimSpace(c.AdminPassword) != "" && isKnownWeakSecret(c.AdminPassword) {
			return fmt.Errorf("ADMIN_PASSWORD must not use the default development password in production")
		}
	}
	return nil
}

func isProduction(appEnv string) bool {
	return strings.EqualFold(strings.TrimSpace(appEnv), "production")
}

func isKnownWeakSecret(value string) bool {
	switch strings.TrimSpace(value) {
	case "admin123456", "change-me-32-byte-encryption-key", "please-change-this-long-random-secret", "development-only-change-me-32-byte-key":
		return true
	default:
		return false
	}
}

func loadEnvFiles() {
	loadEnvFile(".env")
	loadEnvFile("../.env")
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	if appEnv == "" {
		appEnv = "development"
	}
	if strings.EqualFold(appEnv, "development") || getBool("LOAD_DEVELOPMENT_ENV", false) {
		loadEnvFile(".env.development")
		loadEnvFile("../.env.development")
	}
}

func loadEnvFile(path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	_ = godotenv.Load(path)
}

func getString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
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
