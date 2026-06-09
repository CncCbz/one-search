package keypool

import (
	"context"
	"sync"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/provider"
)

type Store interface {
	ListAvailableProviderKeys(ctx context.Context, providerName string) ([]model.APIKey, error)
	RecordKeyResult(ctx context.Context, key model.APIKey, success bool, errorType string) error
}

type Manager struct {
	store     Store
	mu        sync.Mutex
	positions map[string]int
	states    map[int64]*keyState
}

type keyState struct {
	active      int
	windowStart time.Time
	windowCount int
}

func NewManager(store Store) *Manager {
	return &Manager{store: store, positions: map[string]int{}, states: map[int64]*keyState{}}
}

func (m *Manager) Acquire(ctx context.Context, providerName string) (model.APIKey, func(bool, error), error) {
	keys, err := m.store.ListAvailableProviderKeys(ctx, providerName)
	if err != nil {
		return model.APIKey{}, nil, err
	}
	if len(keys) == 0 {
		return model.APIKey{}, nil, &provider.Error{Type: provider.ErrorTypeNoKey, Message: "no available key for " + providerName}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	start := m.positions[providerName]
	for attempt := 0; attempt < len(keys); attempt++ {
		index := (start + attempt) % len(keys)
		key := keys[index]
		state := m.stateFor(key.ID)
		if !m.canUse(state, key, now) {
			continue
		}
		state.active++
		state.windowCount++
		m.positions[providerName] = (index + 1) % len(keys)
		released := false
		release := func(success bool, err error) {
			m.mu.Lock()
			if !released {
				released = true
				if state.active > 0 {
					state.active--
				}
			}
			m.mu.Unlock()
			errorType := provider.ErrorType(err)
			_ = m.store.RecordKeyResult(context.Background(), key, success, errorType)
		}
		return key, release, nil
	}
	return model.APIKey{}, nil, &provider.Error{Type: provider.ErrorTypeRateLimited, Message: "all keys are limited or busy for " + providerName}
}

func (m *Manager) stateFor(keyID int64) *keyState {
	state := m.states[keyID]
	if state == nil {
		state = &keyState{windowStart: time.Now()}
		m.states[keyID] = state
	}
	return state
}

func (m *Manager) canUse(state *keyState, key model.APIKey, now time.Time) bool {
	if key.MaxConcurrency > 0 && state.active >= key.MaxConcurrency {
		return false
	}
	if key.RPMLimit > 0 {
		if now.Sub(state.windowStart) >= time.Minute {
			state.windowStart = now
			state.windowCount = 0
		}
		if state.windowCount >= key.RPMLimit {
			return false
		}
	}
	return true
}
