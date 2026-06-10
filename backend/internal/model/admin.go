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
	MonthlyQuota     int        `json:"monthly_quota"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	UsageCount       int64      `json:"usage_count"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ProviderKeyView struct {
	ID                         int64      `json:"id"`
	ProviderID                 int64      `json:"provider_id"`
	ProviderName               string     `json:"provider_name"`
	Alias                      string     `json:"alias"`
	KeyHint                    string     `json:"key_hint"`
	Key                        string     `json:"key"`
	ExaAPIKeyID                string     `json:"exa_api_key_id,omitempty"`
	ExaServiceKeyHint          string     `json:"exa_service_key_hint,omitempty"`
	Status                     string     `json:"status"`
	Weight                     int        `json:"weight"`
	RPMLimit                   int        `json:"rpm_limit"`
	DailyQuota                 int        `json:"daily_quota"`
	MonthlyQuota               int        `json:"monthly_quota"`
	MaxConcurrency             int        `json:"max_concurrency"`
	CurrentFailures            int        `json:"current_failures"`
	TotalSuccesses             int64      `json:"total_successes"`
	TotalFailures              int64      `json:"total_failures"`
	DailyUsed                  int64      `json:"daily_used"`
	MonthlyUsed                int64      `json:"monthly_used"`
	OfficialQuotaStatus        string     `json:"official_quota_status"`
	OfficialQuotaMessage       string     `json:"official_quota_message"`
	OfficialQuotaUnit          string     `json:"official_quota_unit"`
	OfficialQuotaBalance       *float64   `json:"official_quota_balance,omitempty"`
	OfficialQuotaBalanceUSD    *float64   `json:"official_quota_balance_usd,omitempty"`
	OfficialQuotaUsedUSD       *float64   `json:"official_quota_used_usd,omitempty"`
	OfficialQuotaTotalQuantity *float64   `json:"official_quota_total_quantity,omitempty"`
	OfficialQuotaAccountID     string     `json:"official_quota_account_id,omitempty"`
	OfficialQuotaCheckedAt     *time.Time `json:"official_quota_checked_at,omitempty"`
	CooldownUntil              *time.Time `json:"cooldown_until,omitempty"`
	LastUsedAt                 *time.Time `json:"last_used_at,omitempty"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
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

type UsageMeasurement struct {
	Unit     string                 `json:"unit"`
	Quantity float64                `json:"quantity"`
	CostUSD  *float64               `json:"cost_usd,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ProviderCallLog struct {
	ID            int64              `json:"id,omitempty"`
	ProviderKeyID int64              `json:"provider_key_id"`
	ProviderName  string             `json:"provider_name"`
	KeyAlias      string             `json:"key_alias"`
	AttemptIndex  int                `json:"attempt_index"`
	WillRetry     bool               `json:"will_retry"`
	Status        string             `json:"status"`
	ErrorType     string             `json:"error_type"`
	ErrorMessage  string             `json:"error_message"`
	LatencyMS     int64              `json:"latency_ms"`
	ResultCount   int                `json:"result_count"`
	Cached        bool               `json:"cached"`
	Usage         []UsageMeasurement `json:"usage,omitempty"`
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
	ExaAPIKeyID    *string `json:"exa_api_key_id,omitempty"`
	ExaServiceKey  *string `json:"exa_service_key,omitempty"`
	Status         *string `json:"status,omitempty"`
	Weight         *int    `json:"weight,omitempty"`
	RPMLimit       *int    `json:"rpm_limit,omitempty"`
	DailyQuota     *int    `json:"daily_quota,omitempty"`
	MonthlyQuota   *int    `json:"monthly_quota,omitempty"`
	MaxConcurrency *int    `json:"max_concurrency,omitempty"`
}

// ProviderKeyQuotaResult is the normalized result of a provider's official billing/quota endpoint.
type ProviderKeyQuotaResult struct {
	Provider      string                   `json:"provider"`
	Alias         string                   `json:"alias"`
	Supported     bool                     `json:"supported"`
	Status        string                   `json:"status"`
	Message       string                   `json:"message,omitempty"`
	Unit          string                   `json:"unit,omitempty"`
	Balance       *float64                 `json:"balance,omitempty"`
	BalanceCents  *float64                 `json:"balance_cents,omitempty"`
	BalanceUSD    *float64                 `json:"balance_usd,omitempty"`
	TotalCostUSD  *float64                 `json:"total_cost_usd,omitempty"`
	TotalQuantity *float64                 `json:"total_quantity,omitempty"`
	APIKeyID      string                   `json:"api_key_id,omitempty"`
	APIKeyName    string                   `json:"api_key_name,omitempty"`
	AccountID     string                   `json:"account_id,omitempty"`
	Period        map[string]string        `json:"period,omitempty"`
	Breakdown     []map[string]interface{} `json:"breakdown,omitempty"`
	Raw           map[string]interface{}   `json:"raw,omitempty"`
	RawText       string                   `json:"raw_text,omitempty"`
	FetchedAt     time.Time                `json:"fetched_at"`
}

type ProviderKeyQuotaRequest struct {
	ExaAPIKeyID   string `json:"exa_api_key_id"`
	ExaServiceKey string `json:"exa_service_key"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	GroupBy       string `json:"group_by"`
}

type ProviderHealth struct {
	ProviderName   string    `json:"provider_name"`
	DisplayName    string    `json:"display_name"`
	Enabled        bool      `json:"enabled"`
	Status         string    `json:"status"`
	AvailableKeys  int       `json:"available_keys"`
	TotalKeys      int       `json:"total_keys"`
	ExhaustedKeys  int       `json:"exhausted_keys"`
	DisabledKeys   int       `json:"disabled_keys"`
	CoolingKeys    int       `json:"cooling_keys"`
	RequestsTotal  int64     `json:"requests_total"`
	RequestsFailed int64     `json:"requests_failed"`
	SuccessRate    float64   `json:"success_rate"`
	LastError      string    `json:"last_error"`
	LastCheckedAt  time.Time `json:"last_checked_at"`
	WindowMinutes  int       `json:"window_minutes"`
}

type UsageUnitSummary struct {
	ProviderName  string  `json:"provider_name"`
	Unit          string  `json:"unit"`
	QuantityTotal float64 `json:"quantity_total"`
	CostUSDTotal  float64 `json:"cost_usd_total"`
}

type BillingSummary struct {
	Days  int                `json:"days"`
	Units []UsageUnitSummary `json:"units"`
}

type GatewayMetrics struct {
	Usage          UsageSummary     `json:"usage"`
	ProviderHealth []ProviderHealth `json:"provider_health"`
	Billing        BillingSummary   `json:"billing"`
}

type AuditLog struct {
	ID           int64                  `json:"id"`
	RequestID    string                 `json:"request_id"`
	Actor        string                 `json:"actor"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	IPAddress    string                 `json:"ip_address"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
}

type AuditLogInput struct {
	RequestID    string
	Actor        string
	Action       string
	ResourceType string
	ResourceID   string
	IPAddress    string
	Metadata     map[string]interface{}
}
