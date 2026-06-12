package search

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/provider"
)

type orchestratorTestStore struct {
	settings  model.RuntimeSettings
	providers []model.ProviderConfig
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
	return nil, false, nil
}

func (s *orchestratorTestStore) SetCache(ctx context.Context, cacheKey string, payload []byte, ttlSeconds int) error {
	return nil
}

type orchestratorTestKeyPool struct {
	acquired []string
}

func (p *orchestratorTestKeyPool) Acquire(ctx context.Context, providerName string) (model.APIKey, func(bool, error), error) {
	p.acquired = append(p.acquired, providerName)
	return model.APIKey{ID: int64(len(p.acquired)), ProviderName: providerName, Alias: providerName + "-key", Value: "test-key"}, func(bool, error) {}, nil
}

type orchestratorTestProvider struct {
	name string
}

func (p orchestratorTestProvider) Name() string {
	return p.name
}

func (p orchestratorTestProvider) Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error) {
	return model.ProviderResponse{Results: []model.SearchResult{{Title: p.name, URL: "https://example.com/" + p.name, Provider: p.name, Score: 1}}}, nil
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
