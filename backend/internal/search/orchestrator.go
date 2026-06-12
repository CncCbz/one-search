package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
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
	UpdateProviderKeyOfficialQuota(ctx context.Context, id int64, quota model.ProviderKeyQuotaResult) error
	RuntimeSettings(ctx context.Context) (model.RuntimeSettings, error)
	ListProviders(ctx context.Context) ([]model.ProviderConfig, error)
	RecordSearchLog(ctx context.Context, input model.SearchLogInput) error
	GetCache(ctx context.Context, cacheKey string) ([]byte, bool, error)
	SetCache(ctx context.Context, cacheKey string, payload []byte, ttlSeconds int) error
}

type Orchestrator struct {
	registry       *provider.Registry
	keyPool        KeyPool
	store          Store
	quotaMu        sync.Mutex
	quotaRefreshes map[int64]quotaRefreshState
}

type quotaRefreshState struct {
	inFlight     bool
	lastStarted  time.Time
	lastFinished time.Time
}

func NewOrchestrator(registry *provider.Registry, keyPool KeyPool, store Store) *Orchestrator {
	return &Orchestrator{registry: registry, keyPool: keyPool, store: store, quotaRefreshes: map[int64]quotaRefreshState{}}
}

func (o *Orchestrator) Search(ctx context.Context, req model.SearchRequest, requestID string, apiTokenID int64) (model.SearchResponse, error) {
	started := time.Now()
	settings, err := o.store.RuntimeSettings(ctx)
	if err != nil {
		return model.SearchResponse{}, err
	}
	providerConfigs, err := o.store.ListProviders(ctx)
	if err != nil {
		return model.SearchResponse{}, err
	}
	req = applyDefaults(req, settings)
	if !req.ProvidersExplicit {
		req.Providers = routeProviders(req.Providers, providerConfigs, settings.ProviderRoutingStrategy)
	}
	req.Providers = filterEnabledProviders(req.Providers, providerConfigs)
	providerConfigByName := providerConfigMap(providerConfigs)
	providerSettings := providerSettingsFromProviders(providerConfigs)
	providerLimits := providerResultLimits(providerSettings)
	keyRetryCounts := providerKeyRetryCounts(providerSettings)
	providerTimeouts := providerTimeouts(providerSettings)
	providerProxies := providerProxies(providerSettings)
	providerRetryableErrors := providerRetryableErrors(providerSettings)
	if !req.LimitExplicit && len(req.Providers) == 1 {
		if limit := providerLimits[req.Providers[0]]; limit > 0 {
			req.Limit = limit
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(settings.RequestTimeoutMS)*time.Millisecond)
	defer cancel()

	cacheKey := o.cacheKey(req, providerLimits)
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
		providerResults = o.searchFallback(ctx, req, providerConfigByName, providerLimits, keyRetryCounts, providerTimeouts, providerProxies, providerRetryableErrors)
	case model.SearchModeSingle:
		providerResults = o.searchSingle(ctx, req, providerConfigByName, providerLimits, keyRetryCounts, providerTimeouts, providerProxies, providerRetryableErrors)
	default:
		providerResults = o.searchParallel(ctx, req, providerConfigByName, providerLimits, keyRetryCounts, providerTimeouts, providerProxies, providerRetryableErrors)
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
	responseJSON, _ := json.Marshal(responseLogPayload(response, providerResults))
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

func (o *Orchestrator) searchParallel(ctx context.Context, req model.SearchRequest, providerConfigs map[string]model.ProviderConfig, providerLimits map[string]int, keyRetryCounts map[string]int, providerTimeouts map[string]int, providerProxies map[string]string, retryableErrors map[string]map[string]bool) []providerExecution {
	var wg sync.WaitGroup
	results := make([]providerExecution, len(req.Providers))
	for index, name := range req.Providers {
		wg.Add(1)
		go func(i int, providerName string) {
			defer wg.Done()
			results[i] = o.callProvider(ctx, req, providerName, providerConfigs, providerLimits, keyRetryCounts, providerTimeouts, providerProxies, retryableErrors)
		}(index, name)
	}
	wg.Wait()
	return results
}

func (o *Orchestrator) searchFallback(ctx context.Context, req model.SearchRequest, providerConfigs map[string]model.ProviderConfig, providerLimits map[string]int, keyRetryCounts map[string]int, providerTimeouts map[string]int, providerProxies map[string]string, retryableErrors map[string]map[string]bool) []providerExecution {
	results := []providerExecution{}
	for _, name := range req.Providers {
		execution := o.callProvider(ctx, req, name, providerConfigs, providerLimits, keyRetryCounts, providerTimeouts, providerProxies, retryableErrors)
		results = append(results, execution)
		if execution.err == nil && len(execution.results) > 0 {
			break
		}
	}
	return results
}

func (o *Orchestrator) searchSingle(ctx context.Context, req model.SearchRequest, providerConfigs map[string]model.ProviderConfig, providerLimits map[string]int, keyRetryCounts map[string]int, providerTimeouts map[string]int, providerProxies map[string]string, retryableErrors map[string]map[string]bool) []providerExecution {
	if len(req.Providers) == 0 {
		return nil
	}
	return []providerExecution{o.callProvider(ctx, req, req.Providers[0], providerConfigs, providerLimits, keyRetryCounts, providerTimeouts, providerProxies, retryableErrors)}
}

func (o *Orchestrator) callProvider(ctx context.Context, req model.SearchRequest, providerName string, providerConfigs map[string]model.ProviderConfig, providerLimits map[string]int, keyRetryCounts map[string]int, providerTimeouts map[string]int, providerProxies map[string]string, retryableErrors map[string]map[string]bool) providerExecution {
	started := time.Now()
	execution := providerExecution{provider: providerName, status: "error"}
	adapter, ok := o.adapterForProvider(providerName, providerConfigs[providerName], providerTimeouts[providerName], providerProxies[providerName])
	if !ok {
		err := &provider.Error{Type: provider.ErrorTypeUpstream, Message: "provider is not registered"}
		execution.err = err
		execution.errorType = provider.ErrorType(err)
		execution.latencyMS = time.Since(started).Milliseconds()
		execution.attempts = append(execution.attempts, providerAttempt{
			AttemptIndex: 1,
			Status:       "error",
			ErrorType:    execution.errorType,
			Err:          err,
			LatencyMS:    execution.latencyMS,
		})
		return execution
	}
	providerReq := req
	if limit := providerLimits[providerName]; limit > 0 {
		providerReq.Limit = limit
	}
	retryCount, ok := keyRetryCounts[providerName]
	if !ok {
		retryCount = 3
	}
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > 20 {
		retryCount = 20
	}
	attempts := retryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		attemptStarted := time.Now()
		attemptIndex := attempt + 1
		key, release, err := o.keyPool.Acquire(ctx, providerName)
		if err != nil {
			execution.err = err
			execution.errorType = provider.ErrorType(err)
			execution.latencyMS = time.Since(started).Milliseconds()
			execution.attempts = append(execution.attempts, providerAttempt{
				AttemptIndex: attemptIndex,
				Status:       "error",
				ErrorType:    execution.errorType,
				Err:          err,
				LatencyMS:    time.Since(attemptStarted).Milliseconds(),
			})
			return execution
		}
		callCtx := ctx
		cancel := func() {}
		if timeout := providerTimeouts[providerName]; timeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
		}
		providerResponse, err := adapter.Search(callCtx, providerReq, key)
		cancel()
		success := err == nil
		release(success, err)
		o.refreshOfficialQuota(key)
		attemptLatency := time.Since(attemptStarted).Milliseconds()
		execution.latencyMS = time.Since(started).Milliseconds()
		execution.key = key
		execution.keyAlias = key.Alias
		if err == nil {
			execution.status = "success"
			execution.err = nil
			execution.errorType = ""
			execution.results = providerResponse.Results
			execution.attempts = append(execution.attempts, providerAttempt{
				Key:          key,
				KeyAlias:     key.Alias,
				AttemptIndex: attemptIndex,
				Status:       "success",
				LatencyMS:    attemptLatency,
				ResultCount:  len(providerResponse.Results),
				Usage:        providerResponse.Usage,
			})
			return execution
		}
		willRetry := attempt < attempts-1 && shouldRetryWithNextKey(err, retryableErrors[providerName])
		execution.err = err
		execution.errorType = provider.ErrorType(err)
		execution.attempts = append(execution.attempts, providerAttempt{
			Key:          key,
			KeyAlias:     key.Alias,
			AttemptIndex: attemptIndex,
			WillRetry:    willRetry,
			Status:       "error",
			ErrorType:    execution.errorType,
			Err:          err,
			LatencyMS:    attemptLatency,
		})
		if !willRetry {
			return execution
		}
	}
	return execution
}

func shouldRetryWithNextKey(err error, allowed map[string]bool) bool {
	errorType := provider.ErrorType(err)
	if len(allowed) == 0 {
		switch errorType {
		case provider.ErrorTypeAuth, provider.ErrorTypeQuotaExhausted, provider.ErrorTypeRateLimited:
			return true
		default:
			return false
		}
	}
	return allowed[errorType]
}

func (o *Orchestrator) refreshOfficialQuota(key model.APIKey) {
	if key.ID == 0 || !autoRefreshOfficialQuota(key.ProviderName) {
		return
	}
	now := time.Now()
	interval := quotaRefreshInterval(key.ProviderName)
	o.quotaMu.Lock()
	state := o.quotaRefreshes[key.ID]
	if state.inFlight || (!state.lastStarted.IsZero() && now.Sub(state.lastStarted) < interval) || (!state.lastFinished.IsZero() && now.Sub(state.lastFinished) < interval) {
		o.quotaMu.Unlock()
		return
	}
	state.inFlight = true
	state.lastStarted = now
	o.quotaRefreshes[key.ID] = state
	o.quotaMu.Unlock()

	go func() {
		defer func() {
			o.quotaMu.Lock()
			state := o.quotaRefreshes[key.ID]
			state.inFlight = false
			state.lastFinished = time.Now()
			o.quotaRefreshes[key.ID] = state
			o.quotaMu.Unlock()
		}()
		quota, err := QueryOfficialQuota(context.Background(), key, model.ProviderKeyQuotaRequest{})
		if err != nil {
			quota = model.ProviderKeyQuotaResult{Provider: key.ProviderName, Alias: key.Alias, Supported: true, Status: "error", Message: err.Error(), FetchedAt: time.Now()}
		}
		_ = o.store.UpdateProviderKeyOfficialQuota(context.Background(), key.ID, quota)
	}()
}

func autoRefreshOfficialQuota(providerName string) bool {
	switch providerName {
	case model.ProviderSerper, model.ProviderBrave:
		return false
	default:
		return true
	}
}

func quotaRefreshInterval(providerName string) time.Duration {
	switch providerName {
	case model.ProviderExa:
		return 5 * time.Minute
	case model.ProviderYou, model.ProviderJina, model.ProviderTavily, model.ProviderFirecrawl, model.ProviderSerper, model.ProviderBrave:
		return time.Minute
	default:
		return 5 * time.Minute
	}
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
		req.Providers = append([]string(nil), model.DefaultProviders...)
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

func filterEnabledProviders(providers []string, configs []model.ProviderConfig) []string {
	if len(providers) == 0 || len(configs) == 0 {
		return providers
	}
	configByName := map[string]model.ProviderConfig{}
	for _, item := range configs {
		configByName[item.Name] = item
	}
	filtered := make([]string, 0, len(providers))
	for _, name := range providers {
		if config, ok := configByName[name]; ok && !config.Enabled {
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered
}

func routeProviders(providers []string, configs []model.ProviderConfig, strategy string) []string {
	if len(providers) <= 1 {
		return providers
	}
	configByName := map[string]model.ProviderConfig{}
	for _, item := range configs {
		configByName[item.Name] = item
	}
	ordered := append([]string(nil), providers...)
	switch strategy {
	case "priority":
		sort.SliceStable(ordered, func(i, j int) bool {
			left := configByName[ordered[i]]
			right := configByName[ordered[j]]
			if left.Priority == right.Priority {
				return ordered[i] < ordered[j]
			}
			return left.Priority < right.Priority
		})
	case "weighted":
		sort.SliceStable(ordered, func(i, j int) bool {
			left := configByName[ordered[i]]
			right := configByName[ordered[j]]
			if left.Weight == right.Weight {
				return left.Priority < right.Priority
			}
			return left.Weight > right.Weight
		})
	case "random":
		rand.Shuffle(len(ordered), func(i, j int) { ordered[i], ordered[j] = ordered[j], ordered[i] })
	case "weighted_random":
		return weightedProviderOrder(ordered, configByName)
	case "available_keys":
		sort.SliceStable(ordered, func(i, j int) bool {
			left := configByName[ordered[i]]
			right := configByName[ordered[j]]
			if left.AvailableKeys == right.AvailableKeys {
				return left.Priority < right.Priority
			}
			return left.AvailableKeys > right.AvailableKeys
		})
	}
	return ordered
}

func weightedProviderOrder(providers []string, configByName map[string]model.ProviderConfig) []string {
	remaining := append([]string(nil), providers...)
	ordered := make([]string, 0, len(providers))
	for len(remaining) > 0 {
		totalWeight := 0
		for _, name := range remaining {
			weight := configByName[name].Weight
			if weight <= 0 {
				weight = 1
			}
			totalWeight += weight
		}
		pick := rand.Intn(totalWeight)
		selected := 0
		for index, name := range remaining {
			weight := configByName[name].Weight
			if weight <= 0 {
				weight = 1
			}
			if pick < weight {
				selected = index
				break
			}
			pick -= weight
		}
		ordered = append(ordered, remaining[selected])
		remaining = append(remaining[:selected], remaining[selected+1:]...)
	}
	return ordered
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

func providerConfigMap(providers []model.ProviderConfig) map[string]model.ProviderConfig {
	items := map[string]model.ProviderConfig{}
	for _, item := range providers {
		items[item.Name] = item
	}
	return items
}

func (o *Orchestrator) adapterForProvider(name string, cfg model.ProviderConfig, timeoutMS int, proxyURL string) (provider.Provider, bool) {
	providerCfg := provider.Config{BaseURL: strings.TrimSpace(cfg.BaseURL), ProxyURL: proxyURL}
	if timeoutMS <= 0 {
		timeoutMS = cfg.TimeoutMS
	}
	if timeoutMS > 0 {
		providerCfg.Timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	if adapter, ok := o.registry.Build(name, providerCfg); ok {
		return adapter, true
	}
	return o.registry.Get(name)
}

func providerSettingsFromProviders(providers []model.ProviderConfig) map[string]map[string]interface{} {
	settings := map[string]map[string]interface{}{}
	for _, item := range providers {
		providerSettings := map[string]interface{}{}
		for key, value := range item.Settings {
			providerSettings[key] = value
		}
		providerSettings["_timeout_ms"] = item.TimeoutMS
		settings[item.Name] = providerSettings
	}
	return settings
}

func providerResultLimits(settings map[string]map[string]interface{}) map[string]int {
	limits := map[string]int{}
	for name, item := range settings {
		if limit := intSetting(item, "request_result_limit"); limit > 0 {
			if limit > 50 {
				limit = 50
			}
			limits[name] = limit
		}
	}
	return limits
}

func providerKeyRetryCounts(settings map[string]map[string]interface{}) map[string]int {
	counts := map[string]int{}
	for name, item := range settings {
		count := 3
		if _, ok := item["key_retry_count"]; ok {
			count = intSetting(item, "key_retry_count")
		}
		if count < 0 {
			count = 0
		}
		if count > 20 {
			count = 20
		}
		counts[name] = count
	}
	return counts
}

func providerTimeouts(settings map[string]map[string]interface{}) map[string]int {
	timeouts := map[string]int{}
	for name, item := range settings {
		if timeout := intSetting(item, "_timeout_ms"); timeout > 0 {
			timeouts[name] = timeout
		}
	}
	return timeouts
}

func providerProxies(settings map[string]map[string]interface{}) map[string]string {
	proxies := map[string]string{}
	for name, item := range settings {
		if boolSetting(item, "proxy_enabled") {
			if proxyURL := strings.TrimSpace(stringSetting(item, "proxy_url")); proxyURL != "" {
				proxies[name] = proxyURL
			}
		}
	}
	return proxies
}

func providerRetryableErrors(settings map[string]map[string]interface{}) map[string]map[string]bool {
	result := map[string]map[string]bool{}
	for name, item := range settings {
		values := stringListSetting(item, "retry_error_types")
		if len(values) == 0 {
			continue
		}
		allowed := map[string]bool{}
		for _, value := range values {
			if value != "" {
				allowed[value] = true
			}
		}
		result[name] = allowed
	}
	return result
}

func boolSetting(settings map[string]interface{}, key string) bool {
	if settings == nil {
		return false
	}
	value, ok := settings[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func stringSetting(settings map[string]interface{}, key string) string {
	if settings == nil {
		return ""
	}
	value, ok := settings[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func intSetting(settings map[string]interface{}, key string) int {
	if settings == nil {
		return 0
	}
	value, ok := settings[key]
	if !ok {
		return 0
	}
	result := 0
	switch typed := value.(type) {
	case int:
		result = typed
	case int64:
		result = int(typed)
	case float64:
		result = int(typed)
	case string:
		_, _ = fmt.Sscanf(typed, "%d", &result)
	}
	return result
}

func stringListSetting(settings map[string]interface{}, key string) []string {
	if settings == nil {
		return nil
	}
	value, ok := settings[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		items := []string{}
		for _, item := range typed {
			if text, ok := item.(string); ok {
				items = append(items, strings.TrimSpace(text))
			}
		}
		return items
	case string:
		parts := strings.Split(typed, ",")
		items := []string{}
		for _, part := range parts {
			items = append(items, strings.TrimSpace(part))
		}
		return items
	default:
		return nil
	}
}

func (o *Orchestrator) cacheKey(req model.SearchRequest, providerLimits map[string]int) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"query":           req.Query,
		"providers":       req.Providers,
		"provider_limits": providerLimits,
		"mode":            req.Mode,
		"limit":           req.Limit,
		"freshness":       req.Freshness,
		"dedupe":          req.Dedupe,
		"rerank":          req.Rerank,
		"compat":          req.CompatFormat,
		"options":         req.Options,
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
	attempts  []providerAttempt
}

type providerAttempt struct {
	Key          model.APIKey
	KeyAlias     string
	AttemptIndex int
	WillRetry    bool
	Status       string
	ErrorType    string
	Err          error
	LatencyMS    int64
	ResultCount  int
	Usage        []model.UsageMeasurement
}

type searchResponseLogPayload struct {
	Results         []model.SearchResult        `json:"results"`
	Providers       []model.ProviderCallSummary `json:"providers"`
	Meta            model.SearchMeta            `json:"meta"`
	ProviderResults []providerResultLog         `json:"provider_results,omitempty"`
	ProviderCalls   []model.ProviderCallLog     `json:"provider_calls,omitempty"`
}

type providerResultLog struct {
	Provider    string               `json:"provider"`
	KeyAlias    string               `json:"key_alias,omitempty"`
	Status      string               `json:"status"`
	ErrorType   string               `json:"error_type,omitempty"`
	Error       string               `json:"error,omitempty"`
	LatencyMS   int64                `json:"latency_ms"`
	ResultCount int                  `json:"result_count"`
	Cached      bool                 `json:"cached"`
	Results     []model.SearchResult `json:"results"`
}

func responseLogPayload(response model.SearchResponse, executions []providerExecution) searchResponseLogPayload {
	return searchResponseLogPayload{
		Results:         response.Results,
		Providers:       response.Providers,
		Meta:            response.Meta,
		ProviderResults: providerResultLogs(executions),
		ProviderCalls:   callLogs(executions),
	}
}

func providerResultLogs(executions []providerExecution) []providerResultLog {
	items := make([]providerResultLog, 0, len(executions))
	for _, execution := range executions {
		message := ""
		if execution.err != nil {
			message = execution.err.Error()
		}
		items = append(items, providerResultLog{
			Provider:    execution.provider,
			KeyAlias:    execution.keyAlias,
			Status:      execution.status,
			ErrorType:   execution.errorType,
			Error:       message,
			LatencyMS:   execution.latencyMS,
			ResultCount: len(execution.results),
			Cached:      false,
			Results:     execution.results,
		})
	}
	return items
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
	items := []model.ProviderCallLog{}
	for _, execution := range executions {
		if len(execution.attempts) == 0 {
			message := ""
			if execution.err != nil {
				message = execution.err.Error()
			}
			items = append(items, model.ProviderCallLog{
				ProviderKeyID: execution.key.ID,
				ProviderName:  execution.provider,
				KeyAlias:      execution.keyAlias,
				AttemptIndex:  1,
				Status:        execution.status,
				ErrorType:     execution.errorType,
				ErrorMessage:  message,
				LatencyMS:     execution.latencyMS,
				ResultCount:   len(execution.results),
				Cached:        false,
			})
			continue
		}
		for _, attempt := range execution.attempts {
			message := ""
			if attempt.Err != nil {
				message = attempt.Err.Error()
			}
			attemptIndex := attempt.AttemptIndex
			if attemptIndex <= 0 {
				attemptIndex = 1
			}
			items = append(items, model.ProviderCallLog{
				ProviderKeyID: attempt.Key.ID,
				ProviderName:  execution.provider,
				KeyAlias:      attempt.KeyAlias,
				AttemptIndex:  attemptIndex,
				WillRetry:     attempt.WillRetry,
				Status:        attempt.Status,
				ErrorType:     attempt.ErrorType,
				ErrorMessage:  message,
				LatencyMS:     attempt.LatencyMS,
				ResultCount:   attempt.ResultCount,
				Cached:        false,
				Usage:         attempt.Usage,
			})
		}
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
	providerConfigs, err := o.store.ListProviders(ctx)
	if err != nil {
		return model.ProviderCallSummary{}, nil, err
	}
	providerConfigByName := providerConfigMap(providerConfigs)
	adapter, ok := o.adapterForProvider(key.ProviderName, providerConfigByName[key.ProviderName], 0, "")
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
