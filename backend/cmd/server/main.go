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
	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/provider"
	"github.com/one-search/one-search/backend/internal/search"
	"github.com/one-search/one-search/backend/internal/security"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	log := logging.New()
	cfg, err := config.Load()
	if err != nil {
		log.Error("config_invalid", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
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
	adminExists, err := store.AdminExists(ctx, cfg.AdminUsername)
	if err != nil {
		log.Error("admin_lookup_failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	if !adminExists {
		if cfg.AdminPassword == "" {
			log.Error("admin_password_required", map[string]interface{}{"error": "ADMIN_PASSWORD is required when creating the initial admin user"})
			os.Exit(1)
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Error("admin_password_hash_failed", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
		created, err := store.EnsureAdmin(ctx, cfg.AdminUsername, string(passwordHash))
		if err != nil {
			log.Error("ensure_admin_failed", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
		if created {
			log.Info("admin_created", map[string]interface{}{"username": cfg.AdminUsername})
		}
	}

	registry, err := buildProviderRegistry(cfg)
	if err != nil {
		log.Error("provider_registry_failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	keyPool := keypool.NewManager(store)
	orchestrator := search.NewOrchestrator(registry, keyPool, store)
	auth := api.NewAuthService(store, cfg.AdminSessionTTL, cfg.AdminLoginMaxAttempts, cfg.AdminLoginWindow, cfg.AdminLoginLockout)
	handler := api.NewHandler(store, auth, orchestrator)
	handler.SetLogger(log)
	if cfg.MCPEnabled {
		handler.EnableMCP(cfg.MCPPath)
	}

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
		ReadHeaderTimeout: cfg.ServerReadHeaderTimeout,
		ReadTimeout:       cfg.ServerReadTimeout,
		WriteTimeout:      cfg.ServerWriteTimeout,
		IdleTimeout:       cfg.ServerIdleTimeout,
	}

	go func() {
		log.Info("server_starting", map[string]interface{}{"addr": cfg.HTTPAddr, "mcp_enabled": cfg.MCPEnabled, "mcp_path": cfg.MCPPath})
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

func buildProviderRegistry(cfg config.Config) (*provider.Registry, error) {
	registry := provider.NewRegistry()
	registry.RegisterFactory(model.ProviderExa, func(providerCfg provider.Config) provider.Provider {
		providerCfg.UserAgent = cfg.UpstreamUserAgent
		if providerCfg.Timeout == 0 {
			providerCfg.Timeout = cfg.RequestTimeout
		}
		return provider.NewExaProvider(providerCfg)
	})
	registry.RegisterFactory(model.ProviderYou, func(providerCfg provider.Config) provider.Provider {
		providerCfg.UserAgent = cfg.UpstreamUserAgent
		if providerCfg.Timeout == 0 {
			providerCfg.Timeout = cfg.RequestTimeout
		}
		return provider.NewYouProvider(providerCfg)
	})
	registry.RegisterFactory(model.ProviderJina, func(providerCfg provider.Config) provider.Provider {
		providerCfg.UserAgent = cfg.UpstreamUserAgent
		if providerCfg.Timeout == 0 {
			providerCfg.Timeout = cfg.RequestTimeout
		}
		return provider.NewJinaProvider(providerCfg)
	})
	registry.RegisterFactory(model.ProviderTavily, func(providerCfg provider.Config) provider.Provider {
		providerCfg.UserAgent = cfg.UpstreamUserAgent
		if providerCfg.Timeout == 0 {
			providerCfg.Timeout = cfg.RequestTimeout
		}
		return provider.NewTavilyProvider(providerCfg)
	})
	registry.RegisterFactory(model.ProviderFirecrawl, func(providerCfg provider.Config) provider.Provider {
		providerCfg.UserAgent = cfg.UpstreamUserAgent
		if providerCfg.Timeout == 0 {
			providerCfg.Timeout = cfg.RequestTimeout
		}
		return provider.NewFirecrawlProvider(providerCfg)
	})
	registry.RegisterFactory(model.ProviderSerper, func(providerCfg provider.Config) provider.Provider {
		providerCfg.UserAgent = cfg.UpstreamUserAgent
		if providerCfg.Timeout == 0 {
			providerCfg.Timeout = cfg.RequestTimeout
		}
		return provider.NewSerperProvider(providerCfg)
	})
	registry.RegisterFactory(model.ProviderBrave, func(providerCfg provider.Config) provider.Provider {
		providerCfg.UserAgent = cfg.UpstreamUserAgent
		if providerCfg.Timeout == 0 {
			providerCfg.Timeout = cfg.RequestTimeout
		}
		return provider.NewBraveProvider(providerCfg)
	})
	return registry, nil
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
