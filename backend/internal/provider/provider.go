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

type Factory func(Config) Provider

type Registry struct {
	providers map[string]Provider
	factories map[string]Factory
}

func NewRegistry(items ...Provider) *Registry {
	registry := &Registry{providers: map[string]Provider{}, factories: map[string]Factory{}}
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

func (r *Registry) RegisterFactory(name string, factory Factory) {
	if name == "" || factory == nil {
		return
	}
	r.factories[name] = factory
}

func (r *Registry) Get(name string) (Provider, bool) {
	item, ok := r.providers[name]
	return item, ok
}

func (r *Registry) Build(name string, cfg Config) (Provider, bool) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, false
	}
	return factory(cfg), true
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.providers)+len(r.factories))
	seen := map[string]bool{}
	for name := range r.providers {
		names = append(names, name)
		seen[name] = true
	}
	for name := range r.factories {
		if !seen[name] {
			names = append(names, name)
		}
	}
	return names
}
