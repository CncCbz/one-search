package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/provider"
)

type KeyPool interface {
	Acquire(ctx context.Context, providerName string) (model.APIKey, func(bool, error), error)
}

type Store interface {
	GetAPIKeyByID(ctx context.Context, id int64) (model.APIKey, error)
	RecordKeyResult(ctx context.Context, key model.APIKey, success bool, errorType string) error
	RuntimeSettings(ctx context.Context) (model.RuntimeSettings, error)
	ListProviders(ctx context.Context) ([]model.ProviderConfig, error)
	RecordSearchLog(ctx context.Context, input model.SearchLogInput) error
	GetCache(ctx context.Context, cacheKey string) ([]byte, bool, error)
	SetCache(ctx context.Context, cacheKey string, payload []byte, ttlSeconds int) error
}

type Orchestrator struct {
	registry *provider.Registry
	keyPool  KeyPool
	store    Store
}

func NewOrchestrator(registry *provider.Registry, keyPool KeyPool, store Store) *Orchestrator {
	return &Orchestrator{registry: registry, keyPool: keyPool, store: store}
}

func (o *Orchestrator) Search(ctx context.Context, req model.SearchRequest, requestID string, apiTokenID int64) (model.SearchResponse, error) {
	started := time.Now()
	settings, err := o.store.RuntimeSettings(ctx)
	if err != nil {
		return model.SearchResponse{}, err
	}
	req = applyDefaults(req, settings)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(settings.RequestTimeoutMS)*time.Millisecond)
	defer cancel()

	cacheKey := o.cacheKey(req)
	cacheEnabled := settings.CacheEnabled && req.Cache != model.CachePolicyBypass && req.Cache != model.CachePolicyRefresh
	if cacheEnabled {
		if payload, hit, err := o.store.GetCache(ctx, cacheKey); err != nil {
			return model.SearchResponse{}, err
		} else if hit {
			var cached model.SearchResponse
			if err := json.Unmarshal(payload, &cached); err == nil {
				cached.Meta.CacheHit = true
				cached.Meta.RequestID = requestID
				cached.Meta.LatencyMS = time.Since(started).Milliseconds()
				cached.Meta.CacheKey = cacheKey
				requestJSON, _ := json.Marshal(req)
				responseJSON, _ := json.Marshal(cached)
				_ = o.store.RecordSearchLog(context.Background(), model.SearchLogInput{
					RequestID:    requestID,
					APITokenID:   apiTokenID,
					Query:        req.Query,
					Mode:         string(req.Mode),
					CompatFormat: string(req.CompatFormat),
					Providers:    req.Providers,
					CachePolicy:  string(req.Cache),
					CacheHit:     true,
					ResultCount:  len(cached.Results),
					Status:       "success",
					LatencyMS:    cached.Meta.LatencyMS,
					RequestJSON:  requestJSON,
					ResponseJSON: responseJSON,
				})
				return cached, nil
			}
		}
	}

	var providerResults []providerExecution
	switch req.Mode {
	case model.SearchModeFallback:
		providerResults = o.searchFallback(ctx, req)
	case model.SearchModeSingle:
		providerResults = o.searchSingle(ctx, req)
	default:
		providerResults = o.searchParallel(ctx, req)
	}

	results, deduped := mergeResults(providerResults, req)
	status := "success"
	errorMessage := ""
	if len(results) == 0 && hasOnlyErrors(providerResults) {
		status = "error"
		errorMessage = firstError(providerResults)
	}
	response := model.SearchResponse{
		Results:   results,
		Providers: summaries(providerResults),
		Meta: model.SearchMeta{
			RequestID:        requestID,
			Mode:             req.Mode,
			CompatFormat:     req.CompatFormat,
			LatencyMS:        time.Since(started).Milliseconds(),
			TotalResults:     len(results),
			DedupedResults:   deduped,
			CacheHit:         false,
			CacheKey:         cacheKey,
			ProvidersQueried: providersQueried(providerResults),
		},
	}
	requestJSON, _ := json.Marshal(req)
	responseJSON, _ := json.Marshal(response)
	_ = o.store.RecordSearchLog(context.Background(), model.SearchLogInput{
		RequestID:    requestID,
		APITokenID:   apiTokenID,
		Query:        req.Query,
		Mode:         string(req.Mode),
		CompatFormat: string(req.CompatFormat),
		Providers:    req.Providers,
		CachePolicy:  string(req.Cache),
		CacheHit:     false,
		ResultCount:  len(response.Results),
		Status:       status,
		ErrorMessage: errorMessage,
		LatencyMS:    response.Meta.LatencyMS,
		RequestJSON:  requestJSON,
		ResponseJSON: responseJSON,
		Calls:        callLogs(providerResults),
	})
	if status == "success" && cacheEnabled {
		if payload, err := json.Marshal(response); err == nil {
			_ = o.store.SetCache(context.Background(), cacheKey, payload, settings.CacheTTLSeconds)
		}
	}
	return response, nil
}

func (o *Orchestrator) searchParallel(ctx context.Context, req model.SearchRequest) []providerExecution {
	var wg sync.WaitGroup
	results := make([]providerExecution, len(req.Providers))
	for index, name := range req.Providers {
		wg.Add(1)
		go func(i int, providerName string) {
			defer wg.Done()
			results[i] = o.callProvider(ctx, req, providerName)
		}(index, name)
	}
	wg.Wait()
	return results
}

func (o *Orchestrator) searchFallback(ctx context.Context, req model.SearchRequest) []providerExecution {
	results := []providerExecution{}
	for _, name := range req.Providers {
		execution := o.callProvider(ctx, req, name)
		results = append(results, execution)
		if execution.err == nil && len(execution.results) > 0 {
			break
		}
	}
	return results
}

func (o *Orchestrator) searchSingle(ctx context.Context, req model.SearchRequest) []providerExecution {
	if len(req.Providers) == 0 {
		return nil
	}
	return []providerExecution{o.callProvider(ctx, req, req.Providers[0])}
}

func (o *Orchestrator) callProvider(ctx context.Context, req model.SearchRequest, providerName string) providerExecution {
	started := time.Now()
	execution := providerExecution{provider: providerName, status: "error"}
	adapter, ok := o.registry.Get(providerName)
	if !ok {
		execution.err = &provider.Error{Type: provider.ErrorTypeUpstream, Message: "provider is not registered"}
		execution.errorType = provider.ErrorType(execution.err)
		execution.latencyMS = time.Since(started).Milliseconds()
		return execution
	}
	key, release, err := o.keyPool.Acquire(ctx, providerName)
	if err != nil {
		execution.err = err
		execution.errorType = provider.ErrorType(err)
		execution.latencyMS = time.Since(started).Milliseconds()
		return execution
	}
	execution.key = key
	execution.keyAlias = key.Alias
	providerResponse, err := adapter.Search(ctx, req, key)
	success := err == nil
	release(success, err)
	execution.latencyMS = time.Since(started).Milliseconds()
	if err != nil {
		execution.err = err
		execution.errorType = provider.ErrorType(err)
		return execution
	}
	execution.status = "success"
	execution.results = providerResponse.Results
	return execution
}

func applyDefaults(req model.SearchRequest, settings model.RuntimeSettings) model.SearchRequest {
	req.Query = strings.TrimSpace(req.Query)
	if req.Mode == "" {
		req.Mode = settings.DefaultMode
	}
	if req.Mode == "" {
		req.Mode = model.SearchModeParallel
	}
	if len(req.Providers) == 0 {
		req.Providers = settings.DefaultProviders
	}
	if len(req.Providers) == 0 {
		req.Providers = []string{model.ProviderExa, model.ProviderYou, model.ProviderJina}
	}
	if req.Limit <= 0 {
		req.Limit = settings.DefaultLimit
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 50 {
		req.Limit = 50
	}
	if req.Dedupe == nil {
		req.Dedupe = &settings.DefaultDedupe
	}
	if req.Cache == "" {
		req.Cache = model.CachePolicyDefault
	}
	if req.CompatFormat == "" {
		req.CompatFormat = model.CompatFormatNative
	}
	return req
}

func mergeResults(executions []providerExecution, req model.SearchRequest) ([]model.SearchResult, int) {
	merged := []model.SearchResult{}
	seen := map[string]int{}
	deduped := 0
	dedupe := req.Dedupe == nil || *req.Dedupe
	for _, execution := range executions {
		for _, result := range execution.results {
			canonical := canonicalURL(result.URL)
			if canonical == "" {
				continue
			}
			if dedupe {
				if existingIndex, ok := seen[canonical]; ok {
					existing := &merged[existingIndex]
					existing.Providers = appendUnique(existing.Providers, result.Provider)
					if result.Score > existing.Score {
						existing.Score = result.Score
					}
					deduped++
					continue
				}
				seen[canonical] = len(merged)
			}
			if len(result.Providers) == 0 && result.Provider != "" {
				result.Providers = []string{result.Provider}
			}
			merged = append(merged, result)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Score == merged[j].Score {
			return merged[i].Title < merged[j].Title
		}
		return merged[i].Score > merged[j].Score
	})
	if req.Limit > 0 && len(merged) > req.Limit {
		merged = merged[:req.Limit]
	}
	return merged, deduped
}

func canonicalURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return strings.TrimSpace(raw)
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return strings.TrimRight(parsed.String(), "/")
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (o *Orchestrator) cacheKey(req model.SearchRequest) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"query":     req.Query,
		"providers": req.Providers,
		"mode":      req.Mode,
		"limit":     req.Limit,
		"freshness": req.Freshness,
		"dedupe":    req.Dedupe,
		"rerank":    req.Rerank,
		"compat":    req.CompatFormat,
		"options":   req.Options,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

type providerExecution struct {
	provider  string
	key       model.APIKey
	keyAlias  string
	status    string
	errorType string
	err       error
	latencyMS int64
	results   []model.SearchResult
}

func summaries(executions []providerExecution) []model.ProviderCallSummary {
	items := make([]model.ProviderCallSummary, 0, len(executions))
	for _, execution := range executions {
		message := ""
		if execution.err != nil {
			message = execution.err.Error()
		}
		items = append(items, model.ProviderCallSummary{
			Provider:    execution.provider,
			KeyAlias:    execution.keyAlias,
			Status:      execution.status,
			ErrorType:   execution.errorType,
			Error:       message,
			LatencyMS:   execution.latencyMS,
			ResultCount: len(execution.results),
			Cached:      false,
		})
	}
	return items
}

func callLogs(executions []providerExecution) []model.ProviderCallLog {
	items := make([]model.ProviderCallLog, 0, len(executions))
	for _, execution := range executions {
		message := ""
		if execution.err != nil {
			message = execution.err.Error()
		}
		items = append(items, model.ProviderCallLog{
			ProviderKeyID: execution.key.ID,
			ProviderName:  execution.provider,
			KeyAlias:      execution.keyAlias,
			Status:        execution.status,
			ErrorType:     execution.errorType,
			ErrorMessage:  message,
			LatencyMS:     execution.latencyMS,
			ResultCount:   len(execution.results),
			Cached:        false,
		})
	}
	return items
}

func providersQueried(executions []providerExecution) []string {
	items := make([]string, 0, len(executions))
	for _, execution := range executions {
		items = append(items, execution.provider)
	}
	return items
}

func hasOnlyErrors(executions []providerExecution) bool {
	if len(executions) == 0 {
		return true
	}
	for _, execution := range executions {
		if execution.err == nil {
			return false
		}
	}
	return true
}

func firstError(executions []providerExecution) string {
	for _, execution := range executions {
		if execution.err != nil {
			return execution.err.Error()
		}
	}
	return ""
}

func (o *Orchestrator) TestProviderKey(ctx context.Context, keyID int64, query string, limit int) (model.ProviderCallSummary, []model.SearchResult, error) {
	if query == "" {
		query = "latest AI search API news"
	}
	if limit <= 0 {
		limit = 3
	}
	key, err := o.store.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		return model.ProviderCallSummary{}, nil, err
	}
	adapter, ok := o.registry.Get(key.ProviderName)
	if !ok {
		err := &provider.Error{Type: provider.ErrorTypeUpstream, Message: "provider is not registered"}
		return model.ProviderCallSummary{Provider: key.ProviderName, KeyAlias: key.Alias, Status: "error", ErrorType: provider.ErrorType(err), Error: err.Error()}, nil, err
	}
	started := time.Now()
	req := model.SearchRequest{Query: query, Providers: []string{key.ProviderName}, Mode: model.SearchModeSingle, Limit: limit, Cache: model.CachePolicyBypass}
	providerResponse, err := adapter.Search(ctx, req, key)
	latency := time.Since(started).Milliseconds()
	summary := model.ProviderCallSummary{Provider: key.ProviderName, KeyAlias: key.Alias, LatencyMS: latency, ResultCount: len(providerResponse.Results), Cached: false}
	if err != nil {
		summary.Status = "error"
		summary.ErrorType = provider.ErrorType(err)
		summary.Error = err.Error()
		_ = o.store.RecordKeyResult(context.Background(), key, false, summary.ErrorType)
		return summary, nil, err
	}
	summary.Status = "success"
	_ = o.store.RecordKeyResult(context.Background(), key, true, "")
	return summary, providerResponse.Results, nil
}
