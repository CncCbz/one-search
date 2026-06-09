package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/security"
	"golang.org/x/crypto/bcrypt"
)

type AuthStore interface {
	GetAdminByUsername(ctx context.Context, username string) (model.AdminUser, error)
	FindAPIToken(ctx context.Context, token string) (model.APIToken, error)
	RuntimeSettings(ctx context.Context) (model.RuntimeSettings, error)
}

type AuthService struct {
	store       AuthStore
	sessions    map[string]session
	rateWindows map[int64]rateWindow
	mu          sync.Mutex
}

type session struct {
	Username  string
	ExpiresAt time.Time
}

type rateWindow struct {
	StartedAt time.Time
	Count     int
}

func NewAuthService(store AuthStore) *AuthService {
	return &AuthService{store: store, sessions: map[string]session{}, rateWindows: map[int64]rateWindow{}}
}

func (a *AuthService) Login(ctx context.Context, username, password string) (string, error) {
	user, err := a.store.GetAdminByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", err
	}
	token, err := security.RandomToken("adm_")
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.sessions[token] = session{Username: username, ExpiresAt: time.Now().Add(24 * time.Hour)}
	a.mu.Unlock()
	return token, nil
}

func (a *AuthService) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" || !a.validSession(token) {
			writeError(w, http.StatusUnauthorized, "admin login required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *AuthService) requireAPIToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := a.store.RuntimeSettings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !settings.APIAuthRequired {
			next.ServeHTTP(w, r)
			return
		}
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "api token required")
			return
		}
		apiToken, err := a.store.FindAPIToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid api token")
			return
		}
		if !a.allowToken(apiToken) {
			writeError(w, http.StatusTooManyRequests, "api token rate limit exceeded")
			return
		}
		ctx := context.WithValue(r.Context(), apiTokenIDKey, apiToken.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *AuthService) validSession(token string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(s.ExpiresAt) {
		delete(a.sessions, token)
		return false
	}
	return true
}

func (a *AuthService) allowToken(token model.APIToken) bool {
	if token.RateLimitPerMin <= 0 {
		return true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	window := a.rateWindows[token.ID]
	if window.StartedAt.IsZero() || now.Sub(window.StartedAt) >= time.Minute {
		window = rateWindow{StartedAt: now, Count: 0}
	}
	if window.Count >= token.RateLimitPerMin {
		a.rateWindows[token.ID] = window
		return false
	}
	window.Count++
	a.rateWindows[token.ID] = window
	return true
}
