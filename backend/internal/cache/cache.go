package cache

import (
	"context"
	"encoding/json"

	"github.com/one-search/one-search/backend/internal/model"
)

type Store interface {
	GetCache(ctx context.Context, cacheKey string) ([]byte, bool, error)
	SetCache(ctx context.Context, cacheKey string, payload []byte, ttlSeconds int) error
	DeleteExpiredCache(ctx context.Context) error
}

type Cache struct {
	store Store
}

func New(store Store) *Cache {
	return &Cache{store: store}
}

func (c *Cache) GetSearchResponse(ctx context.Context, key string) (model.SearchResponse, bool, error) {
	payload, hit, err := c.store.GetCache(ctx, key)
	if err != nil || !hit {
		return model.SearchResponse{}, hit, err
	}
	var response model.SearchResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return model.SearchResponse{}, false, err
	}
	return response, true, nil
}

func (c *Cache) SetSearchResponse(ctx context.Context, key string, response model.SearchResponse, ttlSeconds int) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return c.store.SetCache(ctx, key, payload, ttlSeconds)
}

func (c *Cache) DeleteExpired(ctx context.Context) error {
	return c.store.DeleteExpiredCache(ctx)
}
