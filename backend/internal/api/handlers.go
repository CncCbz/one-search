package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/one-search/one-search/backend/internal/compat"
	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/search"
)

type AppStore interface {
	AuthStore
	ListProviders(ctx context.Context) ([]model.ProviderConfig, error)
	UpdateProvider(ctx context.Context, provider model.ProviderConfig) error
	ListProviderKeys(ctx context.Context) ([]model.ProviderKeyView, error)
	GetAPIKeyByID(ctx context.Context, id int64) (model.APIKey, error)
	CreateProviderKey(ctx context.Context, providerName, alias, plainKey, exaAPIKeyID, exaServiceKey string, weight, rpmLimit, dailyQuota, monthlyQuota, maxConcurrency int) (model.ProviderKeyView, error)
	UpdateProviderKeyOfficialQuota(ctx context.Context, id int64, quota model.ProviderKeyQuotaResult) error
	UpdateProviderKeyStatus(ctx context.Context, id int64, status string) error
	UpdateProviderKey(ctx context.Context, id int64, patch model.ProviderKeyUpdate) (model.ProviderKeyView, error)
	DeleteProviderKey(ctx context.Context, id int64) error
	ListAPITokens(ctx context.Context) ([]model.APIToken, error)
	CreateAPIToken(ctx context.Context, name string, scopes []string, allowedProviders []string, rateLimit, dailyQuota, monthlyQuota int) (model.APIToken, string, error)
	UpdateAPITokenStatus(ctx context.Context, id int64, status string) error
	UpdateAPIToken(ctx context.Context, id int64, name string, allowedProviders []string, rateLimit, dailyQuota, monthlyQuota int) error
	DeleteAPIToken(ctx context.Context, id int64) error
	GetAdminAPIKey(ctx context.Context) (model.AdminAPIKey, error)
	RotateAdminAPIKey(ctx context.Context) (model.AdminAPIKey, string, error)
	UpdateRuntimeSettings(ctx context.Context, settings model.RuntimeSettings) error
	ListSearchLogs(ctx context.Context, limit int) ([]model.SearchLog, error)
	GetSearchLog(ctx context.Context, id int64) (model.SearchLog, []model.ProviderCallLog, error)
	GetSearchLogByRequestID(ctx context.Context, requestID string) (model.SearchLog, []model.ProviderCallLog, error)
	UsageSummary(ctx context.Context) (model.UsageSummary, error)
	UsageSummarySince(ctx context.Context, from time.Time) (model.UsageSummary, error)
	BillingSummary(ctx context.Context, days int) (model.BillingSummary, error)
	ProviderHealth(ctx context.Context, windowMinutes int) ([]model.ProviderHealth, error)
	UsageSeries(ctx context.Context, days int) (model.UsageSeries, error)
	UsageSeriesSince(ctx context.Context, from time.Time, granularity string) (model.UsageSeries, error)
	ProviderUsageSeries(ctx context.Context, days int) ([]model.ProviderUsagePoint, error)
	ProviderUsageSeriesSince(ctx context.Context, from time.Time) ([]model.ProviderUsagePoint, error)
	ProviderHealthSeries(ctx context.Context, segmentMinutes, segments int) ([]model.HealthSegmentSeries, error)
	RecordAuditLog(ctx context.Context, input model.AuditLogInput) error
	ListAuditLogs(ctx context.Context, limit int) ([]model.AuditLog, error)
}

type Handler struct {
	store        AppStore
	auth         *AuthService
	orchestrator *search.Orchestrator
	log          requestLogger
	mcpEnabled   bool
	mcpPath      string
}

func NewHandler(store AppStore, auth *AuthService, orchestrator *search.Orchestrator) *Handler {
	return &Handler{store: store, auth: auth, orchestrator: orchestrator}
}

func (h *Handler) SetLogger(log requestLogger) {
	h.log = log
}

func (h *Handler) logInfo(message string, fields map[string]interface{}) {
	if h.log != nil {
		h.log.Info(message, fields)
	}
}

func (h *Handler) logError(message string, fields map[string]interface{}) {
	if h.log != nil {
		h.log.Error(message, fields)
	}
}

func (h *Handler) EnableMCP(path string) {
	h.mcpEnabled = true
	h.mcpPath = strings.TrimSpace(path)
	if h.mcpPath == "" {
		h.mcpPath = "/mcp"
	}
	if !strings.HasPrefix(h.mcpPath, "/") {
		h.mcpPath = "/" + h.mcpPath
	}
}

func (h *Handler) Mount(r chi.Router) {
	if h.mcpEnabled {
		h.mountMCP(r, h.mcpPath)
		if h.mcpPath != "/v1/mcp" {
			h.mountMCP(r, "/v1/mcp")
		}
	}

	r.Route("/v1", func(r chi.Router) {
		r.With(h.auth.requireAPIToken).Post("/search", h.search)
		r.With(h.auth.requireAPIToken).Post("/compat/tavily/search", h.tavilySearch)
		r.With(h.auth.requireAPIToken).Post("/compat/serper/search", h.serperSearch)
		r.With(h.auth.requireAPIToken).Post("/compat/openai/responses-search", h.openAISearch)
		r.With(h.auth.requireAPIToken).Get("/providers", h.providers)
		r.With(h.auth.requireAPIToken).Get("/usage/summary", h.usageSummary)
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Post("/login", h.login)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.requireAdmin)
			r.Post("/logout", h.logout)
			r.Get("/me", h.me)
			r.Get("/dashboard", h.dashboard)
			r.Get("/providers", h.adminProviders)
			r.Get("/providers/health", h.providerHealth)
			r.Patch("/providers/{name}", h.updateProvider)
			r.Get("/keys", h.listKeys)
			r.Post("/keys", h.createKey)
			r.Patch("/keys/{id}", h.updateKey)
			r.Post("/keys/{id}/test", h.testKey)
			r.Post("/keys/{id}/quota", h.keyQuota)
			r.Delete("/keys/{id}", h.deleteKey)
			r.Get("/tokens", h.listTokens)
			r.Post("/tokens", h.createToken)
			r.Patch("/tokens/{id}", h.updateToken)
			r.Delete("/tokens/{id}", h.deleteToken)
			r.Get("/settings", h.getSettings)
			r.Put("/settings", h.updateSettings)
			r.Get("/settings/admin-api-key", h.getAdminAPIKey)
			r.Post("/settings/admin-api-key", h.rotateAdminAPIKey)
			r.Get("/logs", h.logs)
			r.Get("/logs/{id}", h.logDetail)
			r.Get("/usage/summary", h.usageSummary)
			r.Get("/usage/billing", h.billingSummary)
			r.Get("/metrics", h.metrics)
			r.Get("/audit-logs", h.auditLogs)
			r.Post("/playground/search", h.adminSearch)
		})
	})
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var req model.SearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.LimitExplicit = hasJSONField(body, "limit")
	req.ProvidersExplicit = hasJSONField(body, "providers")
	req.CompatFormat = model.CompatFormatNative
	h.runSearch(w, r, req)
}

func (h *Handler) adminSearch(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var req model.SearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.LimitExplicit = hasJSONField(body, "limit")
	req.ProvidersExplicit = hasJSONField(body, "providers")
	req.CompatFormat = model.CompatFormatNative
	requestID := RequestID(r.Context())
	response, err := h.orchestrator.Search(r.Context(), req, requestID, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if log, _, err := h.store.GetSearchLogByRequestID(r.Context(), requestID); err == nil && len(log.ResponseJSON) > 0 {
		var payload map[string]interface{}
		if err := json.Unmarshal(log.ResponseJSON, &payload); err == nil {
			writeJSON(w, http.StatusOK, payload)
			return
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) tavilySearch(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.RuntimeSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !settings.CompatTavilyEnabled {
		writeError(w, http.StatusNotFound, "tavily compatibility endpoint is disabled")
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var req compat.TavilySearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	native := compat.TavilyToNative(req)
	native.LimitExplicit = hasJSONField(body, "max_results")
	native.ProvidersExplicit = hasJSONField(body, "providers")
	response, err := h.orchestrator.Search(r.Context(), native, RequestID(r.Context()), APITokenID(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, compat.TavilyFromNative(req.Query, response))
}

func (h *Handler) serperSearch(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.RuntimeSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !settings.CompatSerperEnabled {
		writeError(w, http.StatusNotFound, "serper compatibility endpoint is disabled")
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var req compat.SerperSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	native := compat.SerperToNative(req)
	native.LimitExplicit = hasJSONField(body, "num")
	native.ProvidersExplicit = hasJSONField(body, "providers")
	response, err := h.orchestrator.Search(r.Context(), native, RequestID(r.Context()), APITokenID(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, compat.SerperFromNative(req, response))
}

func (h *Handler) openAISearch(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.RuntimeSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !settings.CompatOpenAIEnabled {
		writeError(w, http.StatusNotFound, "openai compatibility endpoint is disabled")
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var req compat.OpenAISearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	native := compat.OpenAIToNative(req)
	native.LimitExplicit = hasJSONField(body, "limit")
	native.ProvidersExplicit = hasJSONField(body, "providers")
	response, err := h.orchestrator.Search(r.Context(), native, RequestID(r.Context()), APITokenID(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, compat.OpenAIFromNative(response))
}

func (h *Handler) runSearch(w http.ResponseWriter, r *http.Request, req model.SearchRequest) {
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if token, ok := APIToken(r.Context()); ok {
		filtered, err := applyTokenProviders(req.Providers, token.AllowedProviders)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		req.Providers = filtered
	}
	response, err := h.orchestrator.Search(r.Context(), req, RequestID(r.Context()), APITokenID(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) providers(w http.ResponseWriter, r *http.Request) {
	providers, err := h.store.ListProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"providers": providers})
}

func (h *Handler) usageSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.store.UsageSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) billingSummary(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	summary, err := h.store.BillingSummary(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) providerHealth(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.store.RuntimeSettings(r.Context())
	health, err := h.store.ProviderHealth(r.Context(), settings.ProviderHealthWindowMinutes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"providers": health})
}

func (h *Handler) metrics(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.store.RuntimeSettings(r.Context())
	usage, err := h.store.UsageSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	health, err := h.store.ProviderHealth(r.Context(), settings.ProviderHealthWindowMinutes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	billing, err := h.store.BillingSummary(r.Context(), 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.GatewayMetrics{Usage: usage, ProviderHealth: health, Billing: billing})
}

func (h *Handler) auditLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.store.ListAuditLogs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"logs": items})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	ip := clientIP(r)
	token, expiresAt, err := h.auth.Login(r.Context(), req.Username, req.Password, ip)
	if err != nil {
		reason := "invalid_credentials"
		status := http.StatusUnauthorized
		message := "invalid username or password"
		if errors.Is(err, ErrLoginRateLimited) {
			reason = "rate_limited"
			status = http.StatusTooManyRequests
			message = "too many login attempts"
		}
		h.audit(r, req.Username, "admin.login.failed", "session", "", map[string]interface{}{"username": req.Username, "reason": reason, "ip": ip})
		writeError(w, status, message)
		return
	}
	h.audit(r, req.Username, "admin.login", "session", "", map[string]interface{}{"ip": ip, "expires_at": expiresAt})
	writeJSON(w, http.StatusOK, map[string]interface{}{"token": token, "expires_at": expiresAt})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	loggedOut := h.auth.Logout(bearerToken(r))
	h.audit(r, "admin", "admin.logout", "session", "", map[string]interface{}{"logged_out": loggedOut, "ip": clientIP(r)})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"username": "admin"})
}

func dashboardRangeSpec(raw string) model.DashboardRangeMeta {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "24h":
		// 24 个小时格，贴近 status 页 dense bar
		return model.DashboardRangeMeta{Range: "24h", Label: "近 24 小时", Granularity: "hour", SegmentMinutes: 60, Segments: 24, BillingDays: 1}
	case "today":
		return model.DashboardRangeMeta{Range: "today", Label: "今日", Granularity: "hour", SegmentMinutes: 60, Segments: 0, BillingDays: 1}
	case "7d":
		// 7 天 × 每天 1 格
		return model.DashboardRangeMeta{Range: "7d", Label: "近 7 天", Granularity: "day", SegmentMinutes: 24 * 60, Segments: 7, BillingDays: 7}
	case "30d":
		// 30 天日粒度，OpenAI status 风格
		return model.DashboardRangeMeta{Range: "30d", Label: "近 30 天", Granularity: "day", SegmentMinutes: 24 * 60, Segments: 30, BillingDays: 30}
	default:
		return model.DashboardRangeMeta{Range: "14d", Label: "近 14 天", Granularity: "day", SegmentMinutes: 24 * 60, Segments: 14, BillingDays: 14}
	}
}

func dashboardRangeFrom(spec model.DashboardRangeMeta, now time.Time) time.Time {
	loc := now.Location()
	switch spec.Range {
	case "24h":
		return now.Add(-24 * time.Hour)
	case "today":
		y, m, d := now.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	case "7d":
		return now.Add(-7 * 24 * time.Hour)
	case "30d":
		return now.Add(-30 * 24 * time.Hour)
	default: // 14d
		return now.Add(-14 * 24 * time.Hour)
	}
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	spec := dashboardRangeSpec(r.URL.Query().Get("range"))
	now := time.Now()
	from := dashboardRangeFrom(spec, now)
	if spec.Range == "today" {
		// 今日：从 00:00 到现在，按小时切。
		hours := int(now.Sub(from).Hours()) + 1
		if hours < 1 {
			hours = 1
		}
		if hours > 24 {
			hours = 24
		}
		spec.Segments = hours
	}

	settings, _ := h.store.RuntimeSettings(r.Context())
	summary, _ := h.store.UsageSummarySince(r.Context(), from)
	providers, _ := h.store.ListProviders(r.Context())
	healthWindow := settings.ProviderHealthWindowMinutes
	if healthWindow <= 0 {
		healthWindow = 15
	}
	health, _ := h.store.ProviderHealth(r.Context(), healthWindow)
	billing, _ := h.store.BillingSummary(r.Context(), spec.BillingDays)
	usageSeries, _ := h.store.UsageSeriesSince(r.Context(), from, spec.Granularity)
	usageSeries.Range = spec.Range
	providerSeries, err := h.store.ProviderUsageSeriesSince(r.Context(), from)
	if err != nil || providerSeries == nil {
		providerSeries = []model.ProviderUsagePoint{}
	}
	healthSeries, err := h.store.ProviderHealthSeries(r.Context(), spec.SegmentMinutes, spec.Segments)
	if err != nil || healthSeries == nil {
		healthSeries = []model.HealthSegmentSeries{}
	}
	if providers == nil {
		providers = []model.ProviderConfig{}
	}
	if health == nil {
		health = []model.ProviderHealth{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"range":           spec,
		"usage":           summary,
		"providers":       providers,
		"provider_health": health,
		"billing":         billing,
		"usage_series":    usageSeries,
		"provider_series": providerSeries,
		"health_series":   healthSeries,
	})
}

func (h *Handler) adminProviders(w http.ResponseWriter, r *http.Request) {
	h.providers(w, r)
}

func (h *Handler) updateProvider(w http.ResponseWriter, r *http.Request) {
	var req model.ProviderConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.Name = chi.URLParam(r, "name")
	if err := h.store.UpdateProvider(r.Context(), req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "admin", "provider.update", "provider", req.Name, map[string]interface{}{"enabled": req.Enabled, "timeout_ms": req.TimeoutMS})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.ListProviderKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"keys": keys})
}

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderName   string `json:"provider_name"`
		Alias          string `json:"alias"`
		Key            string `json:"key"`
		ExaAPIKeyID    string `json:"exa_api_key_id"`
		ExaServiceKey  string `json:"exa_service_key"`
		Weight         int    `json:"weight"`
		RPMLimit       int    `json:"rpm_limit"`
		DailyQuota     int    `json:"daily_quota"`
		MonthlyQuota   int    `json:"monthly_quota"`
		MaxConcurrency int    `json:"max_concurrency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.ProviderName == model.ProviderExa && strings.TrimSpace(req.ExaServiceKey) == "" {
		writeError(w, http.StatusBadRequest, "Exa x-api-key is required")
		return
	}
	key, err := h.store.CreateProviderKey(r.Context(), req.ProviderName, req.Alias, req.Key, req.ExaAPIKeyID, req.ExaServiceKey, req.Weight, req.RPMLimit, req.DailyQuota, req.MonthlyQuota, req.MaxConcurrency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "admin", "provider_key.create", "provider_key", strconv.FormatInt(key.ID, 10), map[string]interface{}{"provider": key.ProviderName, "alias": key.Alias})
	writeJSON(w, http.StatusCreated, key)
}

func (h *Handler) updateKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req model.ProviderKeyUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	key, err := h.store.UpdateProviderKey(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "admin", "provider_key.update", "provider_key", strconv.FormatInt(id, 10), map[string]interface{}{"provider": key.ProviderName, "alias": key.Alias, "status": key.Status})
	writeJSON(w, http.StatusOK, key)
}

func (h *Handler) testKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	summary, results, err := h.orchestrator.TestProviderKey(r.Context(), id, req.Query, req.Limit)
	status := http.StatusOK
	if err != nil {
		status = http.StatusBadGateway
	}
	h.audit(r, "admin", "provider_key.test", "provider_key", strconv.FormatInt(id, 10), map[string]interface{}{"status": summary.Status, "error_type": summary.ErrorType})
	writeJSON(w, status, map[string]interface{}{"summary": summary, "results": results})
}

func (h *Handler) deleteKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.DeleteProviderKey(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "admin", "provider_key.delete", "provider_key", strconv.FormatInt(id, 10), nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.store.ListAPITokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tokens": tokens})
}

func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name             string   `json:"name"`
		Scopes           []string `json:"scopes"`
		AllowedProviders []string `json:"allowed_providers"`
		RateLimitPerMin  int      `json:"rate_limit_per_min"`
		DailyQuota       int      `json:"daily_quota"`
		MonthlyQuota     int      `json:"monthly_quota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	token, raw, err := h.store.CreateAPIToken(r.Context(), req.Name, req.Scopes, req.AllowedProviders, req.RateLimitPerMin, req.DailyQuota, req.MonthlyQuota)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "admin", "api_token.create", "api_token", strconv.FormatInt(token.ID, 10), map[string]interface{}{"name": token.Name, "rate_limit_per_min": token.RateLimitPerMin, "daily_quota": token.DailyQuota, "monthly_quota": token.MonthlyQuota})
	writeJSON(w, http.StatusCreated, map[string]interface{}{"token": token, "raw_token": raw})
}

func (h *Handler) updateToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name             string   `json:"name"`
		AllowedProviders []string `json:"allowed_providers"`
		RateLimitPerMin  int      `json:"rate_limit_per_min"`
		DailyQuota       int      `json:"daily_quota"`
		MonthlyQuota     int      `json:"monthly_quota"`
		Status           string   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.Name != "" {
		if err := h.store.UpdateAPIToken(r.Context(), id, req.Name, req.AllowedProviders, req.RateLimitPerMin, req.DailyQuota, req.MonthlyQuota); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.audit(r, "admin", "api_token.update", "api_token", strconv.FormatInt(id, 10), map[string]interface{}{"name": req.Name, "rate_limit_per_min": req.RateLimitPerMin, "daily_quota": req.DailyQuota, "monthly_quota": req.MonthlyQuota})
	} else if req.Status != "" {
		if err := h.store.UpdateAPITokenStatus(r.Context(), id, req.Status); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.audit(r, "admin", "api_token.status", "api_token", strconv.FormatInt(id, 10), map[string]interface{}{"status": req.Status})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) deleteToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.DeleteAPIToken(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "admin", "api_token.delete", "api_token", strconv.FormatInt(id, 10), nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.RuntimeSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *Handler) getAdminAPIKey(w http.ResponseWriter, r *http.Request) {
	key, err := h.store.GetAdminAPIKey(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, key)
}

func (h *Handler) rotateAdminAPIKey(w http.ResponseWriter, r *http.Request) {
	key, _, err := h.store.RotateAdminAPIKey(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, AdminActor(r.Context()), "settings.admin_api_key.rotate", "settings", "admin_api_key", map[string]interface{}{"key_prefix": key.KeyPrefix})
	writeJSON(w, http.StatusCreated, key)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var settings model.RuntimeSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.store.UpdateRuntimeSettings(r.Context(), settings); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "admin", "settings.update", "settings", "runtime", map[string]interface{}{"request_timeout_ms": settings.RequestTimeoutMS, "api_auth_required": settings.APIAuthRequired})
	writeJSON(w, http.StatusOK, settings)
}

func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	logs, err := h.store.ListSearchLogs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"logs": logs})
}

func (h *Handler) logDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	log, calls, err := h.store.GetSearchLog(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"log": log, "calls": calls})
}

func readBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func hasJSONField(body []byte, field string) bool {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	_, ok := payload[field]
	return ok
}

func (h *Handler) audit(r *http.Request, actor, action, resourceType, resourceID string, metadata map[string]interface{}) {
	if action == "" {
		return
	}
	if actor == "" || actor == "admin" {
		actor = AdminActor(r.Context())
	}
	_ = h.store.RecordAuditLog(context.Background(), model.AuditLogInput{
		RequestID:    RequestID(r.Context()),
		Actor:        actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    r.RemoteAddr,
		Metadata:     metadata,
	})
}

func applyTokenProviders(requested, allowed []string) ([]string, error) {
	if len(allowed) == 0 {
		return requested, nil
	}
	allowedSet := map[string]bool{}
	for _, item := range allowed {
		allowedSet[item] = true
	}
	if len(requested) == 0 {
		return allowed, nil
	}
	filtered := []string{}
	for _, item := range requested {
		if !allowedSet[item] {
			return nil, fmt.Errorf("api token is not allowed to request provider %s", item)
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}
