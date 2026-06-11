package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvFilesLetsDevelopmentOverrideBaseEnv(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	withTempWorkingDir(t, map[string]string{
		".env":             "DATABASE_URL=postgres://one_search:prod@localhost:15432/one_search?sslmode=disable\nHTTP_ADDR=:8080\n",
		".env.development": "DATABASE_URL=postgres://one_search:dev@localhost:15432/one_search?sslmode=disable\nHTTP_ADDR=:18080\n",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !strings.Contains(cfg.DatabaseURL, ":dev@") {
		t.Fatalf("DatabaseURL = %q, want development override", cfg.DatabaseURL)
	}
	if cfg.HTTPAddr != ":18080" {
		t.Fatalf("HTTPAddr = %q, want :18080", cfg.HTTPAddr)
	}
}

func TestLoadEnvFilesKeepsExplicitEnvironmentValues(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://one_search:explicit@localhost:15432/one_search?sslmode=disable")
	withTempWorkingDir(t, map[string]string{
		".env":             "DATABASE_URL=postgres://one_search:prod@localhost:15432/one_search?sslmode=disable\n",
		".env.development": "DATABASE_URL=postgres://one_search:dev@localhost:15432/one_search?sslmode=disable\n",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !strings.Contains(cfg.DatabaseURL, ":explicit@") {
		t.Fatalf("DatabaseURL = %q, want explicit env value", cfg.DatabaseURL)
	}
}

func withTempWorkingDir(t *testing.T, files map[string]string) {
	t.Helper()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Fatalf("os.Chdir(%q) error = %v", originalWd, err)
		}
	})

	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", name, err)
		}
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", dir, err)
	}
}
