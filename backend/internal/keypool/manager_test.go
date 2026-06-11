package keypool

import (
	"context"
	"testing"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/provider"
)

type fakeStore struct {
	keys           []model.APIKey
	strategy       string
	maxConcurrency int
}

func (s *fakeStore) ListAvailableProviderKeys(ctx context.Context, providerName string) ([]model.APIKey, error) {
	return append([]model.APIKey(nil), s.keys...), nil
}

func (s *fakeStore) ProviderKeySettings(ctx context.Context, providerName string) (string, int, error) {
	return s.strategy, s.maxConcurrency, nil
}

func (s *fakeStore) RecordKeyResult(ctx context.Context, key model.APIKey, success bool, errorType string) error {
	return nil
}

func TestAcquireAllowsConcurrentUseWhenProviderMaxConcurrencyZero(t *testing.T) {
	manager := NewManager(&fakeStore{keys: []model.APIKey{{ID: 1, ProviderName: model.ProviderYou, MaxConcurrency: 0}}})
	releases := make([]func(bool, error), 0, 5)

	for i := 0; i < 5; i++ {
		key, release, err := manager.Acquire(context.Background(), model.ProviderYou)
		if err != nil {
			t.Fatalf("Acquire #%d returned error: %v", i+1, err)
		}
		if key.ID != 1 {
			t.Fatalf("Acquire #%d key ID = %d, want 1", i+1, key.ID)
		}
		releases = append(releases, release)
	}

	for _, release := range releases {
		release(true, nil)
	}
}

func TestAcquireHonorsProviderMaxConcurrency(t *testing.T) {
	manager := NewManager(&fakeStore{
		keys:           []model.APIKey{{ID: 1, ProviderName: model.ProviderYou}, {ID: 2, ProviderName: model.ProviderYou}},
		maxConcurrency: 1,
	})

	_, release, err := manager.Acquire(context.Background(), model.ProviderYou)
	if err != nil {
		t.Fatalf("first Acquire returned error: %v", err)
	}
	defer release(true, nil)

	_, _, err = manager.Acquire(context.Background(), model.ProviderYou)
	if provider.ErrorType(err) != provider.ErrorTypeRateLimited {
		t.Fatalf("second Acquire error type = %q, want %q (err=%v)", provider.ErrorType(err), provider.ErrorTypeRateLimited, err)
	}
}
