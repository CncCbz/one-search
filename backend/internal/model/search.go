package model

import "time"

const (
	ProviderExa       = "exa"
	ProviderYou       = "you"
	ProviderJina      = "jina"
	ProviderTavily    = "tavily"
	ProviderFirecrawl = "firecrawl"
	ProviderSerper    = "serper"
	ProviderBrave     = "brave"
)

var DefaultProviders = []string{ProviderExa, ProviderYou, ProviderJina, ProviderTavily, ProviderFirecrawl, ProviderSerper, ProviderBrave}

type SearchMode string

const (
	SearchModeParallel SearchMode = "parallel"
	SearchModeFallback SearchMode = "fallback"
	SearchModeSingle   SearchMode = "single"
)

type CachePolicy string

const (
	CachePolicyDefault CachePolicy = "default"
	CachePolicyBypass  CachePolicy = "bypass"
	CachePolicyRefresh CachePolicy = "refresh"
)

type CompatFormat string

const (
	CompatFormatNative CompatFormat = "native"
	CompatFormatTavily CompatFormat = "tavily"
	CompatFormatSerper CompatFormat = "serper"
	CompatFormatOpenAI CompatFormat = "openai"
)

type SearchRequest struct {
	Query             string                 `json:"query"`
	Providers         []string               `json:"providers,omitempty"`
	ProvidersExplicit bool                   `json:"-"`
	Mode              SearchMode             `json:"mode,omitempty"`
	Limit             int                    `json:"limit,omitempty"`
	LimitExplicit     bool                   `json:"-"`
	Freshness         string                 `json:"freshness,omitempty"`
	Dedupe            *bool                  `json:"dedupe,omitempty"`
	Rerank            bool                   `json:"rerank,omitempty"`
	Cache             CachePolicy            `json:"cache,omitempty"`
	IncludeRaw        bool                   `json:"include_raw,omitempty"`
	CompatFormat      CompatFormat           `json:"-"`
	Options           map[string]interface{} `json:"options,omitempty"`
}

type SearchResponse struct {
	Results   []SearchResult        `json:"results"`
	Providers []ProviderCallSummary `json:"providers"`
	Meta      SearchMeta            `json:"meta"`
}

type SearchResult struct {
	Title       string                 `json:"title"`
	URL         string                 `json:"url"`
	Snippet     string                 `json:"snippet,omitempty"`
	Content     string                 `json:"content,omitempty"`
	Provider    string                 `json:"provider"`
	Providers   []string               `json:"providers,omitempty"`
	Score       float64                `json:"score,omitempty"`
	PublishedAt *time.Time             `json:"published_at,omitempty"`
	Raw         map[string]interface{} `json:"raw,omitempty"`
}

type ProviderCallSummary struct {
	Provider    string `json:"provider"`
	KeyAlias    string `json:"key_alias,omitempty"`
	Status      string `json:"status"`
	ErrorType   string `json:"error_type,omitempty"`
	Error       string `json:"error,omitempty"`
	LatencyMS   int64  `json:"latency_ms"`
	ResultCount int    `json:"result_count"`
	Cached      bool   `json:"cached"`
}

type SearchMeta struct {
	RequestID        string       `json:"request_id"`
	Mode             SearchMode   `json:"mode"`
	CompatFormat     CompatFormat `json:"compat_format"`
	LatencyMS        int64        `json:"latency_ms"`
	TotalResults     int          `json:"total_results"`
	DedupedResults   int          `json:"deduped_results"`
	CacheHit         bool         `json:"cache_hit"`
	CacheKey         string       `json:"cache_key,omitempty"`
	ProvidersQueried []string     `json:"providers_queried"`
}

type ProviderResponse struct {
	Results []SearchResult         `json:"results"`
	Usage   []UsageMeasurement     `json:"usage,omitempty"`
	Raw     map[string]interface{} `json:"raw,omitempty"`
}

type ProviderConfig struct {
	ID                  int64                  `json:"id"`
	Name                string                 `json:"name"`
	DisplayName         string                 `json:"display_name"`
	BaseURL             string                 `json:"base_url"`
	Enabled             bool                   `json:"enabled"`
	Priority            int                    `json:"priority"`
	Weight              int                    `json:"weight"`
	TimeoutMS           int                    `json:"timeout_ms"`
	DefaultCacheEnabled bool                   `json:"default_cache_enabled"`
	CacheTTLSeconds     int                    `json:"cache_ttl_seconds"`
	Settings            map[string]interface{} `json:"settings,omitempty"`
	AvailableKeys       int                    `json:"available_keys,omitempty"`
}

type APIKey struct {
	ID                int64     `json:"id"`
	ProviderID        int64     `json:"provider_id"`
	ProviderName      string    `json:"provider_name"`
	Alias             string    `json:"alias"`
	Value             string    `json:"-"`
	KeyHint           string    `json:"key_hint"`
	ExaAPIKeyID       string    `json:"exa_api_key_id,omitempty"`
	ExaServiceKey     string    `json:"-"`
	ExaServiceKeyHint string    `json:"exa_service_key_hint,omitempty"`
	Status            string    `json:"status"`
	Weight            int       `json:"weight"`
	RPMLimit          int       `json:"rpm_limit"`
	DailyQuota        int       `json:"daily_quota"`
	MonthlyQuota      int       `json:"monthly_quota"`
	MonthlyUsed       int64     `json:"monthly_used,omitempty"`
	MonthlyCredits    float64   `json:"monthly_credits,omitempty"`
	MaxConcurrency    int       `json:"max_concurrency"`
	TotalSuccesses    int64     `json:"total_successes"`
	TotalFailures     int64     `json:"total_failures"`
	LastUsedAt        time.Time `json:"last_used_at,omitempty"`
	CooldownUntil     time.Time `json:"cooldown_until,omitempty"`
}

type RuntimeSettings struct {
	DefaultMode                 SearchMode `json:"default_mode"`
	DefaultProviders            []string   `json:"default_providers"`
	DefaultLimit                int        `json:"default_limit"`
	DefaultDedupe               bool       `json:"default_dedupe"`
	RequestTimeoutMS            int        `json:"request_timeout_ms"`
	CacheEnabled                bool       `json:"cache_enabled"`
	CacheTTLSeconds             int        `json:"cache_ttl_seconds"`
	CacheMaxResults             int        `json:"cache_max_results"`
	CompatTavilyEnabled         bool       `json:"compat_tavily_enabled"`
	CompatSerperEnabled         bool       `json:"compat_serper_enabled"`
	CompatOpenAIEnabled         bool       `json:"compat_openai_enabled"`
	APIAuthRequired             bool       `json:"api_auth_required"`
	ProviderHealthWindowMinutes int        `json:"provider_health_window_minutes"`
	ProviderRoutingStrategy     string     `json:"provider_routing_strategy"`
	LogRetentionDays            int        `json:"log_retention_days"`
}

type UsageSummary struct {
	RequestsTotal   int64   `json:"requests_total"`
	RequestsSuccess int64   `json:"requests_success"`
	RequestsFailed  int64   `json:"requests_failed"`
	CacheHits       int64   `json:"cache_hits"`
	ResultsTotal    int64   `json:"results_total"`
	AverageLatency  float64 `json:"average_latency_ms"`
}
