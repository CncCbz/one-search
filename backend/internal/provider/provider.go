package provider

import (
	"context"

	"github.com/one-search/one-search/backend/internal/model"
)

type Provider interface {
	Name() string
	Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error)
	HealthCheck(ctx context.Context, key model.APIKey) error
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(items ...Provider) *Registry {
	registry := &Registry{providers: map[string]Provider{}}
	for _, item := range items {
		registry.Register(item)
	}
	return registry
}

func (r *Registry) Register(item Provider) {
	if item == nil {
		return
	}
	r.providers[item.Name()] = item
}

func (r *Registry) Get(name string) (Provider, bool) {
	item, ok := r.providers[name]
	return item, ok
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
