package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/one-search/one-search/backend/internal/api"
	"github.com/one-search/one-search/backend/internal/config"
	"github.com/one-search/one-search/backend/internal/db"
	"github.com/one-search/one-search/backend/internal/keypool"
	"github.com/one-search/one-search/backend/internal/logging"
	"github.com/one-search/one-search/backend/internal/provider"
	"github.com/one-search/one-search/backend/internal/search"
	"github.com/one-search/one-search/backend/internal/security"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg := config.Load()
	log := logging.New()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database_connect_failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.RunMigrations {
		if err := db.RunMigrations(ctx, pool, cfg.MigrationsDir); err != nil {
			log.Error("migration_failed", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
	}

	crypto := security.NewCrypto(cfg.EncryptionKey)
	store := db.NewStore(pool, crypto)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error("admin_password_hash_failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	if err := store.EnsureAdmin(ctx, cfg.AdminUsername, string(passwordHash)); err != nil {
		log.Error("ensure_admin_failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	registry := provider.NewRegistry(
		provider.NewExaProvider(provider.Config{UserAgent: cfg.UpstreamUserAgent, Timeout: cfg.RequestTimeout}),
		provider.NewYouProvider(provider.Config{UserAgent: cfg.UpstreamUserAgent, Timeout: cfg.RequestTimeout}),
		provider.NewJinaProvider(provider.Config{UserAgent: cfg.UpstreamUserAgent, Timeout: cfg.RequestTimeout}),
	)
	keyPool := keypool.NewManager(store)
	orchestrator := search.NewOrchestrator(registry, keyPool, store)
	auth := api.NewAuthService(store)
	handler := api.NewHandler(store, auth, orchestrator)

	server := api.NewServer(cfg, log)
	server.SetHealth(func() bool {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(pingCtx) == nil
	})
	server.Mount(handler.Mount)
	stopCleaner := startLogRetentionCleaner(store, log)
	defer stopCleaner()

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("server_starting", map[string]interface{}{"addr": cfg.HTTPAddr})
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server_failed", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("server_shutdown_failed", map[string]interface{}{"error": err.Error()})
	}
}

func startLogRetentionCleaner(store *db.Store, log *logging.Logger) func() {
	stop := make(chan struct{})
	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		settings, err := store.RuntimeSettings(ctx)
		if err != nil {
			log.Error("log_retention_settings_failed", map[string]interface{}{"error": err.Error()})
			return
		}
		searchDeleted, auditDeleted, err := store.DeleteOldLogs(ctx, settings.LogRetentionDays)
		if err != nil {
			log.Error("log_retention_cleanup_failed", map[string]interface{}{"error": err.Error(), "retention_days": settings.LogRetentionDays})
			return
		}
		if err := store.DeleteExpiredCache(ctx); err != nil {
			log.Error("cache_cleanup_failed", map[string]interface{}{"error": err.Error()})
		}
		if searchDeleted > 0 || auditDeleted > 0 {
			log.Info("log_retention_cleanup", map[string]interface{}{"retention_days": settings.LogRetentionDays, "search_deleted": searchDeleted, "audit_deleted": auditDeleted})
		}
	}
	go func() {
		run()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run()
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}
