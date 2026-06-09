package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

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
	CreateProviderKey(ctx context.Context, providerName, alias, plainKey string, weight, rpmLimit, dailyQuota, monthlyQuota, maxConcurrency int) (model.ProviderKeyView, error)
	UpdateProviderKeyStatus(ctx context.Context, id int64, status string) error
	UpdateProviderKey(ctx context.Context, id int64, patch model.ProviderKeyUpdate) (model.ProviderKeyView, error)
	DeleteProviderKey(ctx context.Context, id int64) error
	ListAPITokens(ctx context.Context) ([]model.APIToken, error)
	CreateAPIToken(ctx context.Context, name string, scopes []string, rateLimit, dailyQuota int) (model.APIToken, string, error)
	UpdateAPITokenStatus(ctx context.Context, id int64, status string) error
	DeleteAPIToken(ctx context.Context, id int64) error
	UpdateRuntimeSettings(ctx context.Context, settings model.RuntimeSettings) error
	ListSearchLogs(ctx context.Context, limit int) ([]model.SearchLog, error)
	GetSearchLog(ctx context.Context, id int64) (model.SearchLog, []model.ProviderCallLog, error)
	UsageSummary(ctx context.Context) (model.UsageSummary, error)
}

type Handler struct {
	store        AppStore
	auth         *AuthService
	orchestrator *search.Orchestrator
}

func NewHandler(store AppStore, auth *AuthService, orchestrator *search.Orchestrator) *Handler {
	return &Handler{store: store, auth: auth, orchestrator: orchestrator}
}

func (h *Handler) Mount(r chi.Router) {
	r.Route("/v1", func(r chi.Router) {
		r.With(h.auth.requireAPIToken).Post("/search", h.search)
		r.With(h.auth.requireAPIToken).Post("/compat/tavily/search", h.tavilySearch)
		r.With(h.auth.requireAPIToken).Post("/compat/serper/search", h.serperSearch)
		r.With(h.auth.requireAPIToken).Post("/compat/openai/responses-search", h.openAISearch)
		r.Get("/providers", h.providers)
		r.Get("/usage/summary", h.usageSummary)
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Post("/login", h.login)
		r.Group(func(r chi.Router) {
			r.Use(h.auth.requireAdmin)
			r.Get("/me", h.me)
			r.Get("/dashboard", h.dashboard)
			r.Get("/providers", h.adminProviders)
			r.Patch("/providers/{name}", h.updateProvider)
			r.Get("/keys", h.listKeys)
			r.Post("/keys", h.createKey)
			r.Patch("/keys/{id}", h.updateKey)
			r.Post("/keys/{id}/test", h.testKey)
			r.Delete("/keys/{id}", h.deleteKey)
			r.Get("/tokens", h.listTokens)
			r.Post("/tokens", h.createToken)
			r.Patch("/tokens/{id}", h.updateToken)
			r.Delete("/tokens/{id}", h.deleteToken)
			r.Get("/settings", h.getSettings)
			r.Put("/settings", h.updateSettings)
			r.Get("/logs", h.logs)
			r.Get("/logs/{id}", h.logDetail)
			r.Get("/usage/summary", h.usageSummary)
			r.Post("/playground/search", h.adminSearch)
		})
	})
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	var req model.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.CompatFormat = model.CompatFormatNative
	h.runSearch(w, r, req)
}

func (h *Handler) adminSearch(w http.ResponseWriter, r *http.Request) {
	var req model.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.CompatFormat = model.CompatFormatNative
	response, err := h.orchestrator.Search(r.Context(), req, RequestID(r.Context()), 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
	var req compat.TavilySearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	native := compat.TavilyToNative(req)
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
	var req compat.SerperSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	native := compat.SerperToNative(req)
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
	var req compat.OpenAISearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	native := compat.OpenAIToNative(req)
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

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	token, err := h.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"username": "admin"})
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	summary, _ := h.store.UsageSummary(r.Context())
	providers, _ := h.store.ListProviders(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{"usage": summary, "providers": providers})
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
	key, err := h.store.CreateProviderKey(r.Context(), req.ProviderName, req.Alias, req.Key, req.Weight, req.RPMLimit, req.DailyQuota, req.MonthlyQuota, req.MaxConcurrency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
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
		Name            string   `json:"name"`
		Scopes          []string `json:"scopes"`
		RateLimitPerMin int      `json:"rate_limit_per_min"`
		DailyQuota      int      `json:"daily_quota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	token, raw, err := h.store.CreateAPIToken(r.Context(), req.Name, req.Scopes, req.RateLimitPerMin, req.DailyQuota)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"token": token, "raw_token": raw})
}

func (h *Handler) updateToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.store.UpdateAPITokenStatus(r.Context(), id, req.Status); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
