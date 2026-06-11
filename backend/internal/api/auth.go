package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/security"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrLoginRateLimited   = errors.New("admin login rate limit exceeded")
)

type AuthStore interface {
	GetAdminByUsername(ctx context.Context, username string) (model.AdminUser, error)
	FindAdminAPIKey(ctx context.Context, token string) (model.AdminAPIKey, bool, error)
	FindAPIToken(ctx context.Context, token string) (model.APIToken, error)
	RuntimeSettings(ctx context.Context) (model.RuntimeSettings, error)
}

type AuthService struct {
	store            AuthStore
	sessions         map[string]session
	rateWindows      map[int64]rateWindow
	loginWindows     map[string]loginWindow
	sessionTTL       time.Duration
	loginMaxAttempts int
	loginWindow      time.Duration
	loginLockout     time.Duration
	mu               sync.Mutex
}

type session struct {
	Username  string
	ExpiresAt time.Time
}

type rateWindow struct {
	StartedAt time.Time
	Count     int
}

type loginWindow struct {
	StartedAt   time.Time
	Count       int
	LockedUntil time.Time
}

func NewAuthService(store AuthStore, sessionTTL time.Duration, loginMaxAttempts int, loginWindowDuration, loginLockout time.Duration) *AuthService {
	if sessionTTL <= 0 {
		sessionTTL = 24 * time.Hour
	}
	if loginMaxAttempts == 0 {
		loginMaxAttempts = 5
	}
	if loginWindowDuration <= 0 {
		loginWindowDuration = 5 * time.Minute
	}
	if loginLockout <= 0 {
		loginLockout = 15 * time.Minute
	}
	return &AuthService{
		store:            store,
		sessions:         map[string]session{},
		rateWindows:      map[int64]rateWindow{},
		loginWindows:     map[string]loginWindow{},
		sessionTTL:       sessionTTL,
		loginMaxAttempts: loginMaxAttempts,
		loginWindow:      loginWindowDuration,
		loginLockout:     loginLockout,
	}
}

func (a *AuthService) Login(ctx context.Context, username, password, clientIP string) (string, time.Time, error) {
	attemptKey := loginAttemptKey(username, clientIP)
	if a.loginLocked(attemptKey) {
		return "", time.Time{}, ErrLoginRateLimited
	}
	user, err := a.store.GetAdminByUsername(ctx, username)
	if err != nil {
		if a.recordLoginFailure(attemptKey) {
			return "", time.Time{}, ErrLoginRateLimited
		}
		return "", time.Time{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		if a.recordLoginFailure(attemptKey) {
			return "", time.Time{}, ErrLoginRateLimited
		}
		return "", time.Time{}, ErrInvalidCredentials
	}
	token, err := security.RandomToken("adm_")
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(a.sessionTTL)
	a.mu.Lock()
	delete(a.loginWindows, attemptKey)
	a.sessions[token] = session{Username: username, ExpiresAt: expiresAt}
	a.mu.Unlock()
	return token, expiresAt, nil
}

func (a *AuthService) Logout(token string) bool {
	if token == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.sessions[token]; !ok {
		return false
	}
	delete(a.sessions, token)
	return true
}

func (a *AuthService) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token != "" && a.validSession(token) {
			ctx := context.WithValue(r.Context(), adminActorKey, "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if token != "" {
			adminKey, ok, err := a.store.FindAdminAPIKey(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if ok {
				ctx := context.WithValue(r.Context(), adminActorKey, adminAPIKeyActor(adminKey))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		writeError(w, http.StatusUnauthorized, "admin login required")
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
		adminKey, ok, err := a.store.FindAdminAPIKey(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if ok {
			ctx := context.WithValue(r.Context(), adminActorKey, adminAPIKeyActor(adminKey))
			next.ServeHTTP(w, r.WithContext(ctx))
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
		ctx = context.WithValue(ctx, apiTokenKey, apiToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func adminAPIKeyActor(key model.AdminAPIKey) string {
	if key.KeyPrefix == "" {
		return "admin_api_key"
	}
	return "admin_api_key:" + key.KeyPrefix
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

func (a *AuthService) loginLocked(key string) bool {
	if a.loginMaxAttempts < 0 {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	window := a.loginWindows[key]
	if window.LockedUntil.After(now) {
		return true
	}
	if !window.LockedUntil.IsZero() && !window.LockedUntil.After(now) {
		delete(a.loginWindows, key)
	}
	return false
}

func (a *AuthService) recordLoginFailure(key string) bool {
	if a.loginMaxAttempts < 0 {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	window := a.loginWindows[key]
	if window.StartedAt.IsZero() || now.Sub(window.StartedAt) > a.loginWindow {
		window = loginWindow{StartedAt: now}
	}
	window.Count++
	if window.Count >= a.loginMaxAttempts {
		window.LockedUntil = now.Add(a.loginLockout)
		a.loginWindows[key] = window
		return true
	}
	a.loginWindows[key] = window
	return false
}

func loginAttemptKey(username, clientIP string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "|" + strings.TrimSpace(clientIP)
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
