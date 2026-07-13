package billing

import (
	"math"
	"strconv"
	"strings"
)

// 公开价目表默认值（PAYG 口径，仅作本地估算）。
// 可在渠道 settings 覆盖：price_per_request / price_per_credit / price_per_token / default_billable_credits
//
// 来源（2026 公开页，会变）：
// - Exa Search: $7 / 1k requests → $0.007/req
// - You.com Web Search: $5 / 1k calls → $0.005/req
// - Tavily: $0.008 / credit（PAYG）
// - Serper: starter ≈ $1 / 1k → $0.001
// - Brave Search: $5 / 1k → $0.005/req
// - Firecrawl Search: 2 credits / 10 results；≈$0.00083/credit
// - Jina: ≈ $0.05 / 1M tokens；Search fallback ≈$0.0005/req

type Rate struct {
	PerRequest            float64 `json:"price_per_request,omitempty"`
	PerCredit             float64 `json:"price_per_credit,omitempty"`
	PerToken              float64 `json:"price_per_token,omitempty"`
	DefaultBillableCredit float64 `json:"default_billable_credits,omitempty"`
}

var defaultRates = map[string]Rate{
	"exa":       {PerRequest: 0.007},
	"you":       {PerRequest: 0.005},
	"tavily":    {PerRequest: 0.008, PerCredit: 0.008, DefaultBillableCredit: 1},
	"serper":    {PerRequest: 0.001, PerCredit: 0.001, DefaultBillableCredit: 1},
	"brave":     {PerRequest: 0.005},
	"firecrawl": {PerRequest: 0.00166, PerCredit: 0.00083, DefaultBillableCredit: 2},
	"jina":      {PerRequest: 0.0005, PerToken: 0.05 / 1_000_000},
}

func DefaultRate(provider string) Rate {
	if rate, ok := defaultRates[strings.ToLower(strings.TrimSpace(provider))]; ok {
		return rate
	}
	return Rate{}
}

// MergeRate overlays positive custom fields onto defaults. Zero custom means keep default.
func MergeRate(base, custom Rate) Rate {
	out := base
	if custom.PerRequest > 0 {
		out.PerRequest = custom.PerRequest
	}
	if custom.PerCredit > 0 {
		out.PerCredit = custom.PerCredit
	}
	if custom.PerToken > 0 {
		out.PerToken = custom.PerToken
	}
	if custom.DefaultBillableCredit > 0 {
		out.DefaultBillableCredit = custom.DefaultBillableCredit
	}
	return out
}

// RateFromSettings reads provider.settings pricing fields.
func RateFromSettings(provider string, settings map[string]interface{}) Rate {
	base := DefaultRate(provider)
	if settings == nil {
		return base
	}
	custom := Rate{
		PerRequest:            floatSetting(settings, "price_per_request"),
		PerCredit:             floatSetting(settings, "price_per_credit"),
		PerToken:              floatSetting(settings, "price_per_token"),
		DefaultBillableCredit: floatSetting(settings, "default_billable_credits"),
	}
	return MergeRate(base, custom)
}

func floatSetting(settings map[string]interface{}, key string) float64 {
	value, ok := settings[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

// EstimateCostUSD uses built-in defaults for provider.
func EstimateCostUSD(provider, unit string, quantity float64) (float64, bool) {
	return EstimateCostUSDWithRate(DefaultRate(provider), unit, quantity)
}

// EstimateCostUSDWithRate estimates USD from an explicit rate table.
func EstimateCostUSDWithRate(rate Rate, unit string, quantity float64) (float64, bool) {
	if quantity <= 0 || math.IsNaN(quantity) || math.IsInf(quantity, 0) {
		return 0, false
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "usd":
		return quantity, true
	case "credits":
		if rate.PerCredit > 0 {
			return quantity * rate.PerCredit, true
		}
	case "tokens":
		if rate.PerToken > 0 {
			return quantity * rate.PerToken, true
		}
	case "requests", "request", "calls", "call":
		if rate.PerRequest > 0 {
			return quantity * rate.PerRequest, true
		}
	}
	return 0, false
}

// DefaultRequestCredits returns default billable credits when upstream omits usage.
func DefaultRequestCredits(provider string) float64 {
	return DefaultRate(provider).DefaultBillableCredit
}

func DefaultRequestCreditsWithRate(rate Rate) float64 {
	return rate.DefaultBillableCredit
}
