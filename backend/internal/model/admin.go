package model

import (
	"encoding/json"
	"time"
)

type AdminUser struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type APIToken struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	TokenHash        string     `json:"-"`
	TokenCiphertext  string     `json:"-"`
	Token            string     `json:"token"`
	TokenPrefix      string     `json:"token_prefix"`
	Scopes           []string   `json:"scopes"`
	AllowedProviders []string   `json:"allowed_providers"`
	Status           string     `json:"status"`
	RateLimitPerMin  int        `json:"rate_limit_per_min"`
	DailyQuota       int        `json:"daily_quota"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	UsageCount       int64      `json:"usage_count"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ProviderKeyView struct {
	ID              int64      `json:"id"`
	ProviderID      int64      `json:"provider_id"`
	ProviderName    string     `json:"provider_name"`
	Alias           string     `json:"alias"`
	KeyHint         string     `json:"key_hint"`
	Key             string     `json:"key"`
	Status          string     `json:"status"`
	Weight          int        `json:"weight"`
	RPMLimit        int        `json:"rpm_limit"`
	DailyQuota      int        `json:"daily_quota"`
	MonthlyQuota    int        `json:"monthly_quota"`
	MaxConcurrency  int        `json:"max_concurrency"`
	CurrentFailures int        `json:"current_failures"`
	TotalSuccesses  int64      `json:"total_successes"`
	TotalFailures   int64      `json:"total_failures"`
	DailyUsed       int64      `json:"daily_used"`
	MonthlyUsed     int64      `json:"monthly_used"`
	CooldownUntil   *time.Time `json:"cooldown_until,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type SearchLog struct {
	ID           int64           `json:"id"`
	RequestID    string          `json:"request_id"`
	Query        string          `json:"query"`
	Mode         string          `json:"mode"`
	CompatFormat string          `json:"compat_format"`
	Providers    []string        `json:"providers"`
	CachePolicy  string          `json:"cache_policy"`
	CacheHit     bool            `json:"cache_hit"`
	ResultCount  int             `json:"result_count"`
	Status       string          `json:"status"`
	ErrorMessage string          `json:"error_message"`
	LatencyMS    int             `json:"latency_ms"`
	RequestJSON  json.RawMessage `json:"request_json,omitempty"`
	ResponseJSON json.RawMessage `json:"response_json,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

type ProviderCallLog struct {
	ProviderKeyID int64  `json:"provider_key_id"`
	ProviderName  string `json:"provider_name"`
	KeyAlias      string `json:"key_alias"`
	Status        string `json:"status"`
	ErrorType     string `json:"error_type"`
	ErrorMessage  string `json:"error_message"`
	LatencyMS     int64  `json:"latency_ms"`
	ResultCount   int    `json:"result_count"`
	Cached        bool   `json:"cached"`
}

type SearchLogInput struct {
	RequestID    string
	APITokenID   int64
	Query        string
	Mode         string
	CompatFormat string
	Providers    []string
	CachePolicy  string
	CacheHit     bool
	ResultCount  int
	Status       string
	ErrorMessage string
	LatencyMS    int64
	RequestJSON  []byte
	ResponseJSON []byte
	Calls        []ProviderCallLog
}

type ProviderKeyUpdate struct {
	Alias          *string `json:"alias,omitempty"`
	Key            *string `json:"key,omitempty"`
	Status         *string `json:"status,omitempty"`
	Weight         *int    `json:"weight,omitempty"`
	RPMLimit       *int    `json:"rpm_limit,omitempty"`
	DailyQuota     *int    `json:"daily_quota,omitempty"`
	MonthlyQuota   *int    `json:"monthly_quota,omitempty"`
	MaxConcurrency *int    `json:"max_concurrency,omitempty"`
}
