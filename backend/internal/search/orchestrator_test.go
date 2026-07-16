package search

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/provider"
)

type orchestratorTestStore struct {
	mu        sync.Mutex
	settings  model.RuntimeSettings
	providers []model.ProviderConfig
	cache     map[string][]byte
	lastTTL   int
	setCount  int
}

func (s *orchestratorTestStore) GetAPIKeyByID(ctx context.Context, id int64) (model.APIKey, error) {
	return model.APIKey{}, nil
}

func (s *orchestratorTestStore) RecordKeyResult(ctx context.Context, key model.APIKey, success bool, errorType string) error {
	return nil
}

func (s *orchestratorTestStore) UpdateProviderKeyOfficialQuota(ctx context.Context, id int64, quota model.ProviderKeyQuotaResult) error {
	return nil
}

func (s *orchestratorTestStore) RuntimeSettings(ctx context.Context) (model.RuntimeSettings, error) {
	return s.settings, nil
}

func (s *orchestratorTestStore) ListProviders(ctx context.Context) ([]model.ProviderConfig, error) {
	return append([]model.ProviderConfig(nil), s.providers...), nil
}

func (s *orchestratorTestStore) RecordSearchLog(ctx context.Context, input model.SearchLogInput) error {
	return nil
}

func (s *orchestratorTestStore) GetCache(ctx context.Context, cacheKey string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		return nil, false, nil
	}
	payload, ok := s.cache[cacheKey]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), payload...), true, nil
}

func (s *orchestratorTestStore) SetCache(ctx context.Context, cacheKey string, payload []byte, ttlSeconds int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		s.cache = map[string][]byte{}
	}
	s.cache[cacheKey] = append([]byte(nil), payload...)
	s.lastTTL = ttlSeconds
	s.setCount++
	return nil
}

type orchestratorTestKeyPool struct {
	mu       sync.Mutex
	acquired []string
}

func (p *orchestratorTestKeyPool) Acquire(ctx context.Context, providerName string) (model.APIKey, func(bool, error), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.acquired = append(p.acquired, providerName)
	return model.APIKey{ID: int64(len(p.acquired)), ProviderName: providerName, Alias: providerName + "-key", Value: "test-key"}, func(bool, error) {}, nil
}

type orchestratorTestProvider struct {
	name       string
	delay      time.Duration
	err        error
	resultN    int
	empty      bool
	searchHook func()
}

func (p orchestratorTestProvider) Name() string {
	return p.name
}

func (p orchestratorTestProvider) Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error) {
	if p.searchHook != nil {
		p.searchHook()
	}
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return model.ProviderResponse{}, ctx.Err()
		}
	}
	if p.err != nil {
		return model.ProviderResponse{}, p.err
	}
	if p.empty {
		return model.ProviderResponse{}, nil
	}
	n := p.resultN
	if n <= 0 {
		n = 1
	}
	results := make([]model.SearchResult, 0, n)
	for i := 0; i < n; i++ {
		results = append(results, model.SearchResult{
			Title:    p.name,
			URL:      "https://example.com/" + p.name + "/" + string(rune('a'+i)),
			Provider: p.name,
			Score:    1,
		})
	}
	return model.ProviderResponse{Results: results}, nil
}

func (p orchestratorTestProvider) HealthCheck(ctx context.Context, key model.APIKey) error {
	return nil
}

func TestSearchSkipsDisabledDefaultProviders(t *testing.T) {
	keyPool := &orchestratorTestKeyPool{}
	orchestrator := newDisabledProviderTestOrchestrator(keyPool)

	response, err := orchestrator.Search(context.Background(), model.SearchRequest{Query: "golang"}, "request-id", 0)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if got, want := keyPool.acquired, []string{model.ProviderSerper}; !stringSlicesEqual(got, want) {
		t.Fatalf("Acquire providers = %v, want %v", got, want)
	}
	if got, want := response.Meta.ProvidersQueried, []string{model.ProviderSerper}; !stringSlicesEqual(got, want) {
		t.Fatalf("ProvidersQueried = %v, want %v", got, want)
	}
	if len(response.Providers) != 1 || response.Providers[0].Provider != model.ProviderSerper {
		t.Fatalf("response providers = %+v, want only %s", response.Providers, model.ProviderSerper)
	}
}

func TestSearchSkipsExplicitDisabledProviders(t *testing.T) {
	keyPool := &orchestratorTestKeyPool{}
	orchestrator := newDisabledProviderTestOrchestrator(keyPool)

	response, err := orchestrator.Search(context.Background(), model.SearchRequest{Query: "golang", Providers: []string{model.ProviderExa}, ProvidersExplicit: true}, "request-id", 0)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if len(keyPool.acquired) != 0 {
		t.Fatalf("Acquire providers = %v, want none", keyPool.acquired)
	}
	if len(response.Meta.ProvidersQueried) != 0 {
		t.Fatalf("ProvidersQueried = %v, want none", response.Meta.ProvidersQueried)
	}
	if len(response.Providers) != 0 {
		t.Fatalf("response providers = %+v, want none", response.Providers)
	}
}

func TestFilterEnabledProvidersKeepsUnknownProviders(t *testing.T) {
	providers := filterEnabledProviders([]string{model.ProviderExa, "custom"}, []model.ProviderConfig{{Name: model.ProviderExa, Enabled: false}})
	if got, want := providers, []string{"custom"}; !stringSlicesEqual(got, want) {
		t.Fatalf("filterEnabledProviders = %v, want %v", got, want)
	}
}

func TestApplyDefaultsDoesNotCapLimitAtFifty(t *testing.T) {
	request := applyDefaults(model.SearchRequest{Query: "golang", Limit: 120}, model.RuntimeSettings{DefaultLimit: 10, DefaultDedupe: true})
	if got, want := request.Limit, 120; got != want {
		t.Fatalf("Limit = %d, want %d", got, want)
	}
}

func TestSearchCacheHitAndRefresh(t *testing.T) {
	keyPool := &orchestratorTestKeyPool{}
	registry := provider.NewRegistry(orchestratorTestProvider{name: model.ProviderSerper})
	store := &orchestratorTestStore{
		settings: model.RuntimeSettings{
			DefaultMode:      model.SearchModeSingle,
			DefaultProviders: []string{model.ProviderSerper},
			DefaultLimit:     10,
			DefaultDedupe:    true,
			RequestTimeoutMS: 1000,
			CacheEnabled:     true,
			CacheTTLSeconds:  60,
		},
		providers: []model.ProviderConfig{
			{Name: model.ProviderSerper, Enabled: true, Priority: 1, Weight: 1},
		},
		cache: map[string][]byte{},
	}
	orchestrator := NewOrchestrator(registry, keyPool, store)
	req := model.SearchRequest{Query: "golang", Providers: []string{model.ProviderSerper}, ProvidersExplicit: true, Mode: model.SearchModeSingle}

	first, err := orchestrator.Search(context.Background(), req, "req-1", 0)
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	if first.Meta.CacheHit {
		t.Fatal("first search should miss cache")
	}
	if len(store.cache) != 1 {
		t.Fatalf("cache entries = %d, want 1", len(store.cache))
	}
	acquiredAfterWrite := len(keyPool.acquired)

	second, err := orchestrator.Search(context.Background(), req, "req-2", 0)
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if !second.Meta.CacheHit {
		t.Fatal("second search should hit cache")
	}
	if len(keyPool.acquired) != acquiredAfterWrite {
		t.Fatalf("cache hit should not acquire keys, acquired=%v", keyPool.acquired)
	}

	// provider order must not affect key
	reordered := req
	reordered.Providers = []string{model.ProviderSerper}
	third, err := orchestrator.Search(context.Background(), reordered, "req-3", 0)
	if err != nil {
		t.Fatalf("reordered search: %v", err)
	}
	if !third.Meta.CacheHit {
		t.Fatal("reordered providers should still hit cache")
	}

	refresh := req
	refresh.Cache = model.CachePolicyRefresh
	fourth, err := orchestrator.Search(context.Background(), refresh, "req-4", 0)
	if err != nil {
		t.Fatalf("refresh search: %v", err)
	}
	if fourth.Meta.CacheHit {
		t.Fatal("refresh should bypass read")
	}
	if len(keyPool.acquired) <= acquiredAfterWrite {
		t.Fatal("refresh should call providers")
	}
	if len(store.cache) != 1 {
		t.Fatalf("refresh should rewrite cache, entries=%d", len(store.cache))
	}
}

func TestCacheKeyStableAcrossProviderOrder(t *testing.T) {
	o := &Orchestrator{}
	left := o.cacheKey(model.SearchRequest{Query: "q", Providers: []string{"b", "a"}, Mode: model.SearchModeParallel, Limit: 10}, map[string]int{"a": 5, "b": 8, "c": 9})
	right := o.cacheKey(model.SearchRequest{Query: "q", Providers: []string{"a", "b"}, Mode: model.SearchModeParallel, Limit: 10}, map[string]int{"a": 5, "b": 8, "c": 9})
	if left != right {
		t.Fatalf("cache keys differ: %s vs %s", left, right)
	}
}

func TestCacheTruncatesResultsOnWrite(t *testing.T) {
	store := &orchestratorTestStore{
		settings: model.RuntimeSettings{
			DefaultMode: model.SearchModeSingle, DefaultProviders: []string{model.ProviderSerper}, DefaultLimit: 10,
			DefaultDedupe: true, RequestTimeoutMS: 1000, CacheEnabled: true, CacheTTLSeconds: 60, CacheMaxResults: 2,
		},
		providers: []model.ProviderConfig{{Name: model.ProviderSerper, Enabled: true, Priority: 1, Weight: 1}},
		cache:     map[string][]byte{},
	}
	orchestrator := NewOrchestrator(provider.NewRegistry(orchestratorTestProvider{name: model.ProviderSerper, resultN: 5}), &orchestratorTestKeyPool{}, store)
	resp, err := orchestrator.Search(context.Background(), model.SearchRequest{Query: "q", Providers: []string{model.ProviderSerper}, ProvidersExplicit: true, Mode: model.SearchModeSingle}, "r1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 5 {
		t.Fatalf("live response results = %d, want 5", len(resp.Results))
	}
	if len(store.cache) != 1 {
		t.Fatalf("cache entries = %d, want 1", len(store.cache))
	}
	var cached model.SearchResponse
	for _, payload := range store.cache {
		if err := json.Unmarshal(payload, &cached); err != nil {
			t.Fatal(err)
		}
	}
	if len(cached.Results) != 2 {
		t.Fatalf("cached results = %d, want 2", len(cached.Results))
	}
}

func TestCacheSkipsPartialParallelErrors(t *testing.T) {
	store := &orchestratorTestStore{
		settings: model.RuntimeSettings{
			DefaultMode: model.SearchModeParallel, DefaultProviders: []string{model.ProviderSerper, model.ProviderBrave},
			DefaultLimit: 10, DefaultDedupe: true, RequestTimeoutMS: 1000, CacheEnabled: true, CacheTTLSeconds: 60,
		},
		providers: []model.ProviderConfig{
			{Name: model.ProviderSerper, Enabled: true, Priority: 1, Weight: 1},
			{Name: model.ProviderBrave, Enabled: true, Priority: 2, Weight: 1},
		},
		cache: map[string][]byte{},
	}
	orchestrator := NewOrchestrator(provider.NewRegistry(
		orchestratorTestProvider{name: model.ProviderSerper},
		orchestratorTestProvider{name: model.ProviderBrave, err: context.DeadlineExceeded},
	), &orchestratorTestKeyPool{}, store)
	resp, err := orchestrator.Search(context.Background(), model.SearchRequest{
		Query: "q", Providers: []string{model.ProviderSerper, model.ProviderBrave}, ProvidersExplicit: true, Mode: model.SearchModeParallel,
	}, "r1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected partial results")
	}
	if len(store.cache) != 0 {
		t.Fatalf("partial parallel errors should not cache, entries=%d", len(store.cache))
	}
}

func TestCacheEmptyResultsUseShortTTL(t *testing.T) {
	store := &orchestratorTestStore{
		settings: model.RuntimeSettings{
			DefaultMode: model.SearchModeSingle, DefaultProviders: []string{model.ProviderSerper}, DefaultLimit: 10,
			DefaultDedupe: true, RequestTimeoutMS: 1000, CacheEnabled: true, CacheTTLSeconds: 3600,
		},
		providers: []model.ProviderConfig{{Name: model.ProviderSerper, Enabled: true, Priority: 1, Weight: 1}},
		cache:     map[string][]byte{},
	}
	orchestrator := NewOrchestrator(provider.NewRegistry(orchestratorTestProvider{name: model.ProviderSerper, empty: true}), &orchestratorTestKeyPool{}, store)
	if _, err := orchestrator.Search(context.Background(), model.SearchRequest{Query: "q", Providers: []string{model.ProviderSerper}, ProvidersExplicit: true, Mode: model.SearchModeSingle}, "r1", 0); err != nil {
		t.Fatal(err)
	}
	if store.setCount != 1 {
		t.Fatalf("setCount=%d, want 1", store.setCount)
	}
	if store.lastTTL != emptyResultCacheTTLSeconds {
		t.Fatalf("empty ttl=%d, want %d", store.lastTTL, emptyResultCacheTTLSeconds)
	}
}

func TestSearchSingleflightCoalesces(t *testing.T) {
	store := &orchestratorTestStore{
		settings: model.RuntimeSettings{
			DefaultMode: model.SearchModeSingle, DefaultProviders: []string{model.ProviderSerper}, DefaultLimit: 10,
			DefaultDedupe: true, RequestTimeoutMS: 2000, CacheEnabled: true, CacheTTLSeconds: 60,
		},
		providers: []model.ProviderConfig{{Name: model.ProviderSerper, Enabled: true, Priority: 1, Weight: 1}},
		cache:     map[string][]byte{},
	}
	keyPool := &orchestratorTestKeyPool{}
	orchestrator := NewOrchestrator(provider.NewRegistry(orchestratorTestProvider{name: model.ProviderSerper, delay: 80 * time.Millisecond}), keyPool, store)
	req := model.SearchRequest{Query: "coalesce", Providers: []string{model.ProviderSerper}, ProvidersExplicit: true, Mode: model.SearchModeSingle}

	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("req-%d", i)
		go func(requestID string) {
			_, err := orchestrator.Search(context.Background(), req, requestID, 0)
			done <- err
		}(id)
	}
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if len(keyPool.acquired) != 1 {
		t.Fatalf("singleflight should acquire once, acquired=%v", keyPool.acquired)
	}
}

func TestProviderResultLimitsDoNotCapAtFifty(t *testing.T) {
	limits := providerResultLimits(map[string]map[string]interface{}{
		model.ProviderExa: {"request_result_limit": 120},
	})
	if got, want := limits[model.ProviderExa], 120; got != want {
		t.Fatalf("provider limit = %d, want %d", got, want)
	}
}

func TestEffectiveRequestTimeoutUsesProviderTimeoutWhenLarger(t *testing.T) {
	got := effectiveRequestTimeoutMS(20000, map[string]int{model.ProviderFirecrawl: 60000}, []string{model.ProviderFirecrawl})
	if want := 61000; got != want {
		t.Fatalf("effective timeout = %d, want %d", got, want)
	}
}

func TestSearchProviderTimeoutCanExceedRuntimeTimeout(t *testing.T) {
	keyPool := &orchestratorTestKeyPool{}
	registry := provider.NewRegistry(orchestratorTestProvider{name: model.ProviderFirecrawl, delay: 60 * time.Millisecond})
	store := &orchestratorTestStore{
		settings: model.RuntimeSettings{
			DefaultMode:      model.SearchModeSingle,
			DefaultProviders: []string{model.ProviderFirecrawl},
			DefaultLimit:     10,
			DefaultDedupe:    true,
			RequestTimeoutMS: 20,
		},
		providers: []model.ProviderConfig{
			{Name: model.ProviderFirecrawl, Enabled: true, Priority: 1, Weight: 1, TimeoutMS: 100},
		},
	}
	orchestrator := NewOrchestrator(registry, keyPool, store)

	response, err := orchestrator.Search(context.Background(), model.SearchRequest{Query: "golang"}, "request-id", 0)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(response.Results) != 1 || response.Providers[0].Status != "success" {
		t.Fatalf("response = %+v, want successful provider result", response)
	}
}

func newDisabledProviderTestOrchestrator(keyPool *orchestratorTestKeyPool) *Orchestrator {
	registry := provider.NewRegistry(orchestratorTestProvider{name: model.ProviderExa}, orchestratorTestProvider{name: model.ProviderSerper})
	store := &orchestratorTestStore{
		settings: model.RuntimeSettings{
			DefaultMode:      model.SearchModeParallel,
			DefaultProviders: []string{model.ProviderExa, model.ProviderSerper},
			DefaultLimit:     10,
			DefaultDedupe:    true,
			RequestTimeoutMS: 1000,
		},
		providers: []model.ProviderConfig{
			{Name: model.ProviderExa, Enabled: false, Priority: 1, Weight: 1},
			{Name: model.ProviderSerper, Enabled: true, Priority: 2, Weight: 1},
		},
	}
	return NewOrchestrator(registry, keyPool, store)
}

func stringSlicesEqual(left, right []string) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}
