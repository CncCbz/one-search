package keypool

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/provider"
)

type Store interface {
	ListAvailableProviderKeys(ctx context.Context, providerName string) ([]model.APIKey, error)
	ProviderKeyRoutingStrategy(ctx context.Context, providerName string) (string, error)
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
	rand.Seed(time.Now().UnixNano())
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

	strategy, err := m.store.ProviderKeyRoutingStrategy(ctx, providerName)
	if err != nil {
		return model.APIKey{}, nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	keys = m.orderKeys(providerName, keys, strategy)
	start := m.startIndex(providerName, strategy)
	for attempt := 0; attempt < len(keys); attempt++ {
		index := (start + attempt) % len(keys)
		key := keys[index]
		state := m.stateFor(key.ID)
		if !m.canUse(state, key, now) {
			continue
		}
		state.active++
		state.windowCount++
		if m.usesPosition(strategy) {
			m.positions[providerName] = (index + 1) % len(keys)
		}
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

func (m *Manager) orderKeys(providerName string, keys []model.APIKey, strategy string) []model.APIKey {
	ordered := append([]model.APIKey(nil), keys...)
	switch strategy {
	case "least_used":
		sort.SliceStable(ordered, func(i, j int) bool {
			left := ordered[i].TotalSuccesses + ordered[i].TotalFailures
			right := ordered[j].TotalSuccesses + ordered[j].TotalFailures
			if left == right {
				if ordered[i].LastUsedAt.Equal(ordered[j].LastUsedAt) {
					return ordered[i].ID < ordered[j].ID
				}
				return ordered[i].LastUsedAt.Before(ordered[j].LastUsedAt)
			}
			return left < right
		})
	case "random":
		rand.Shuffle(len(ordered), func(i, j int) { ordered[i], ordered[j] = ordered[j], ordered[i] })
	case "weighted_random":
		return weightedKeyOrder(ordered)
	default:
		return ordered
	}
	m.positions[providerName] = 0
	return ordered
}

func (m *Manager) startIndex(providerName, strategy string) int {
	if !m.usesPosition(strategy) {
		return 0
	}
	return m.positions[providerName]
}

func (m *Manager) usesPosition(strategy string) bool {
	switch strategy {
	case "least_used", "random", "weighted_random":
		return false
	default:
		return true
	}
}

func weightedKeyOrder(keys []model.APIKey) []model.APIKey {
	remaining := append([]model.APIKey(nil), keys...)
	ordered := make([]model.APIKey, 0, len(keys))
	for len(remaining) > 0 {
		totalWeight := 0
		for _, key := range remaining {
			weight := key.Weight
			if weight <= 0 {
				weight = 1
			}
			totalWeight += weight
		}
		pick := rand.Intn(totalWeight)
		selected := 0
		for index, key := range remaining {
			weight := key.Weight
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
