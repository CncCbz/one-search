package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/provider"
)

var (
	exaUsageBaseURL         = "https://admin-api.exa.ai/team-management/api-keys"
	youQuotaURL             = "https://api.you.com/v1/billing/account_balance"
	jinaQuotaURL            = "https://r.jina.ai/"
	tavilyUsageURL          = "https://api.tavily.com/usage"
	firecrawlCreditUsageURL = "https://api.firecrawl.dev/v2/team/credit-usage"
	braveWebSearchURL       = "https://api.search.brave.com/res/v1/web/search"
	quotaRequestTimeout     = 20 * time.Second
)

// QueryOfficialQuota queries the upstream provider's official quota/billing endpoint.
func QueryOfficialQuota(ctx context.Context, key model.APIKey, req model.ProviderKeyQuotaRequest) (model.ProviderKeyQuotaResult, error) {
	ctx, cancel := context.WithTimeout(ctx, quotaRequestTimeout)
	defer cancel()
	switch key.ProviderName {
	case model.ProviderExa:
		return queryExaQuota(ctx, key, req)
	case model.ProviderYou:
		return queryYouQuota(ctx, key, req.ProxyURL)
	case model.ProviderJina:
		return queryJinaQuota(ctx, key, req.ProxyURL)
	case model.ProviderTavily:
		return queryTavilyQuota(ctx, key, req.ProxyURL)
	case model.ProviderFirecrawl:
		return queryFirecrawlQuota(ctx, key, req.ProxyURL)
	case model.ProviderSerper:
		return querySerperQuota(ctx, key)
	case model.ProviderBrave:
		return queryBraveQuota(ctx, key, req.ProxyURL)
	default:
		return model.ProviderKeyQuotaResult{Provider: key.ProviderName, Alias: key.Alias, Supported: false, Status: "unsupported", Message: "该渠道暂未配置官方额度查询", FetchedAt: time.Now()}, nil
	}
}

func queryExaQuota(ctx context.Context, key model.APIKey, req model.ProviderKeyQuotaRequest) (model.ProviderKeyQuotaResult, error) {
	apiKeyID := strings.TrimSpace(req.ExaAPIKeyID)
	if apiKeyID == "" {
		apiKeyID = strings.TrimSpace(key.ExaAPIKeyID)
	}
	if apiKeyID == "" {
		apiKeyID = strings.TrimSpace(key.Value)
	}
	serviceKey := strings.TrimSpace(req.ExaServiceKey)
	if serviceKey == "" {
		serviceKey = strings.TrimSpace(key.ExaServiceKey)
	}
	if apiKeyID == "" || serviceKey == "" {
		return model.ProviderKeyQuotaResult{}, fmt.Errorf("Exa 官方 usage 查询需要 API Key 和 Team Management x-api-key")
	}
	endpoint := strings.TrimRight(exaUsageBaseURL, "/") + "/" + url.PathEscape(apiKeyID) + "/usage"
	params := url.Values{}
	if strings.TrimSpace(req.StartDate) != "" {
		params.Set("start_date", strings.TrimSpace(req.StartDate))
	}
	if strings.TrimSpace(req.EndDate) != "" {
		params.Set("end_date", strings.TrimSpace(req.EndDate))
	}
	if strings.TrimSpace(req.GroupBy) != "" {
		params.Set("group_by", strings.TrimSpace(req.GroupBy))
	}
	if encoded := params.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-api-key", serviceKey)
	payload, err := doJSONQuotaRequest(httpReq, req.ProxyURL)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	period := map[string]string{}
	if periodMap, ok := payload["period"].(map[string]interface{}); ok {
		period["start"] = stringFromAny(periodMap["start"])
		period["end"] = stringFromAny(periodMap["end"])
	}
	breakdown := objectArrayFromAny(payload["cost_breakdown"])
	totalCost := floatFromAny(payload["total_cost_usd"])
	totalQuantity := 0.0
	for _, item := range breakdown {
		totalQuantity += floatFromAny(item["quantity"])
	}
	return model.ProviderKeyQuotaResult{
		Provider:      key.ProviderName,
		Alias:         key.Alias,
		Supported:     true,
		Status:        "success",
		Unit:          "usd_used",
		Message:       "Exa 官方接口返回指定周期用量/费用，非账户剩余额度",
		TotalCostUSD:  floatPtr(totalCost),
		TotalQuantity: floatPtr(totalQuantity),
		APIKeyID:      stringFromAny(payload["api_key_id"]),
		APIKeyName:    stringFromAny(payload["api_key_name"]),
		Period:        period,
		Breakdown:     breakdown,
		Raw:           payload,
		FetchedAt:     time.Now(),
	}, nil
}

func queryYouQuota(ctx context.Context, key model.APIKey, proxyURL string) (model.ProviderKeyQuotaResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, youQuotaURL, nil)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-API-Key", key.Value)
	payload, err := doJSONQuotaRequest(httpReq, proxyURL)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	data, _ := payload["data"].(map[string]interface{})
	attributes, _ := data["attributes"].(map[string]interface{})
	balanceCents := floatFromAny(attributes["balance"])
	balanceUSD := balanceCents / 100
	return model.ProviderKeyQuotaResult{
		Provider:     key.ProviderName,
		Alias:        key.Alias,
		Supported:    true,
		Status:       "success",
		Unit:         "cents",
		Balance:      floatPtr(balanceCents),
		BalanceCents: floatPtr(balanceCents),
		BalanceUSD:   floatPtr(balanceUSD),
		AccountID:    stringFromAny(data["id"]),
		Raw:          payload,
		FetchedAt:    time.Now(),
	}, nil
}

func queryJinaQuota(ctx context.Context, key model.APIKey, proxyURL string) (model.ProviderKeyQuotaResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, jinaQuotaURL, nil)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	httpReq.Header.Set("Accept", "text/plain")
	httpReq.Header.Set("Authorization", "Bearer "+key.Value)
	client := quotaHTTPClient(proxyURL)
	resp, err := client.Do(httpReq)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	text := string(body)
	if resp.StatusCode >= 400 {
		return model.ProviderKeyQuotaResult{}, fmt.Errorf("Jina quota query failed: status %d: %s", resp.StatusCode, truncateMessage(text, 500))
	}
	balanceMatch := regexp.MustCompile(`(?m)^\[Balance left\]\s+([+-]?[0-9]+(?:\.[0-9]+)?)\s*$`).FindStringSubmatch(text)
	if len(balanceMatch) < 2 {
		return model.ProviderKeyQuotaResult{Provider: key.ProviderName, Alias: key.Alias, Supported: false, Status: "unsupported", Message: "Jina 未提供稳定 JSON 额度接口，且根地址响应中未找到 Balance left", RawText: text, FetchedAt: time.Now()}, nil
	}
	balance, _ := strconv.ParseFloat(balanceMatch[1], 64)
	accountID := ""
	accountMatch := regexp.MustCompile(`(?m)^\[Authenticated as\]\s+(.+?)\s*$`).FindStringSubmatch(text)
	if len(accountMatch) >= 2 {
		accountID = strings.TrimSpace(accountMatch[1])
	}
	return model.ProviderKeyQuotaResult{
		Provider:  key.ProviderName,
		Alias:     key.Alias,
		Supported: true,
		Status:    "success",
		Unit:      "tokens",
		Balance:   floatPtr(balance),
		AccountID: accountID,
		RawText:   text,
		FetchedAt: time.Now(),
	}, nil
}

func queryTavilyQuota(ctx context.Context, key model.APIKey, proxyURL string) (model.ProviderKeyQuotaResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, tavilyUsageURL, nil)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+key.Value)
	payload, err := doJSONQuotaRequest(httpReq, proxyURL)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	keyUsage, _ := payload["key"].(map[string]interface{})
	accountUsage, _ := payload["account"].(map[string]interface{})
	used := firstPositiveFloat(keyUsage, "usage", "search_usage")
	limit := firstPositiveFloat(keyUsage, "limit")
	balance := limit - used
	if limit <= 0 {
		planLimit := firstPositiveFloat(accountUsage, "plan_limit")
		paygoLimit := firstPositiveFloat(accountUsage, "paygo_limit")
		planUsage := firstPositiveFloat(accountUsage, "plan_usage")
		paygoUsage := firstPositiveFloat(accountUsage, "paygo_usage")
		limit = planLimit + paygoLimit
		used = planUsage + paygoUsage
		balance = limit - used
	}
	return model.ProviderKeyQuotaResult{
		Provider:      key.ProviderName,
		Alias:         key.Alias,
		Supported:     true,
		Status:        "success",
		Unit:          "credits",
		Balance:       floatPtr(balance),
		TotalQuantity: floatPtr(used),
		APIKeyName:    stringFromAny(keyUsage["name"]),
		AccountID:     stringFromAny(accountUsage["current_plan"]),
		Message:       "Tavily 官方 /usage 返回当前 API Key 用量和限额",
		Breakdown:     quotaBreakdown("key", keyUsage, "account", accountUsage),
		Raw:           payload,
		FetchedAt:     time.Now(),
	}, nil
}

func queryFirecrawlQuota(ctx context.Context, key model.APIKey, proxyURL string) (model.ProviderKeyQuotaResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, firecrawlCreditUsageURL, nil)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+key.Value)
	payload, err := doJSONQuotaRequest(httpReq, proxyURL)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	data, _ := payload["data"].(map[string]interface{})
	remaining := firstNumber(data, "remainingCredits", "remaining_credits")
	planCredits := firstNumber(data, "planCredits", "plan_credits")
	used := planCredits - remaining
	period := map[string]string{}
	if start := stringFromAny(data["billingPeriodStart"]); start != "" {
		period["start"] = start
	}
	if end := stringFromAny(data["billingPeriodEnd"]); end != "" {
		period["end"] = end
	}
	return model.ProviderKeyQuotaResult{
		Provider:      key.ProviderName,
		Alias:         key.Alias,
		Supported:     true,
		Status:        "success",
		Unit:          "credits",
		Balance:       floatPtr(remaining),
		TotalQuantity: floatPtr(used),
		Message:       "Firecrawl 官方 /v2/team/credit-usage 返回团队剩余 credits",
		Period:        period,
		Breakdown:     quotaBreakdown("data", data),
		Raw:           payload,
		FetchedAt:     time.Now(),
	}, nil
}

const serperDefaultCredits = 2500

func querySerperQuota(ctx context.Context, key model.APIKey) (model.ProviderKeyQuotaResult, error) {
	// 只用本地 credits meter，不用 requests 次数冒充 credits。
	used := key.MonthlyCredits
	if used < 0 {
		used = 0
	}
	balance := float64(serperDefaultCredits) - used
	return model.ProviderKeyQuotaResult{
		Provider:      key.ProviderName,
		Alias:         key.Alias,
		Supported:     true,
		Status:        "success",
		Unit:          "credits",
		Balance:       floatPtr(balance),
		TotalQuantity: floatPtr(used),
		Message:       "Serper 未公开独立余额接口；按默认总额度 2500 credits 减本地累计 credits 估算剩余额度（非官方余额）",
		Breakdown:     quotaBreakdown("default", map[string]interface{}{"credits": serperDefaultCredits}, "local", map[string]interface{}{"monthly_credits": key.MonthlyCredits, "monthly_requests": key.MonthlyUsed}),
		FetchedAt:     time.Now(),
	}, nil
}

func queryBraveQuota(ctx context.Context, key model.APIKey, proxyURL string) (model.ProviderKeyQuotaResult, error) {
	endpoint := braveWebSearchURL + "?q=" + url.QueryEscape("brave quota check") + "&count=1"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-Subscription-Token", key.Value)
	payload, header, err := doJSONQuotaRequestWithHeader(httpReq, proxyURL)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	windows := parseRateLimitWindows(header)
	monthly := quotaWindowByLargestDuration(windows)
	balance := monthly.Remaining
	limit := monthly.Limit
	used := limit - balance
	return model.ProviderKeyQuotaResult{
		Provider:      key.ProviderName,
		Alias:         key.Alias,
		Supported:     true,
		Status:        "success",
		Unit:          "requests",
		Balance:       floatPtr(balance),
		TotalQuantity: floatPtr(used),
		Message:       "Brave 官方通过搜索响应 X-RateLimit-* headers 返回剩余请求额度；查询本身会消耗一次成功请求",
		Breakdown:     rateLimitBreakdown(windows),
		Raw:           payload,
		RawText:       rateLimitRawText(header),
		FetchedAt:     time.Now(),
	}, nil
}

func doJSONQuotaRequest(req *http.Request, proxyURL string) (map[string]interface{}, error) {
	payload, _, err := doJSONQuotaRequestWithHeader(req, proxyURL)
	return payload, err
}

func doJSONQuotaRequestWithHeader(req *http.Request, proxyURL string) (map[string]interface{}, http.Header, error) {
	client := quotaHTTPClient(proxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, resp.Header, err
	}
	if resp.StatusCode >= 400 {
		return nil, resp.Header, fmt.Errorf("official quota query failed: status %d: %s", resp.StatusCode, truncateMessage(string(body), 500))
	}
	var payload map[string]interface{}
	if len(strings.TrimSpace(string(body))) == 0 {
		payload = map[string]interface{}{}
	} else if err := json.Unmarshal(body, &payload); err != nil {
		return nil, resp.Header, fmt.Errorf("decode official quota response: %w", err)
	}
	return payload, resp.Header, nil
}

func quotaHTTPClient(proxyURL string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if normalized := provider.NormalizeProxyURL(proxyURL); normalized != "" {
		if parsed, err := url.Parse(normalized); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return &http.Client{Timeout: quotaRequestTimeout, Transport: transport}
}

type rateLimitWindow struct {
	Limit     float64
	Remaining float64
	Reset     float64
	Duration  int
}

func firstPositiveFloat(values map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		value := floatFromAny(values[key])
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNumber(values map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return floatFromAny(value)
		}
	}
	return 0
}

func quotaBreakdown(parts ...interface{}) []map[string]interface{} {
	breakdown := []map[string]interface{}{}
	for i := 0; i+1 < len(parts); i += 2 {
		name := fmt.Sprint(parts[i])
		values, ok := parts[i+1].(map[string]interface{})
		if !ok || len(values) == 0 {
			continue
		}
		item := map[string]interface{}{"scope": name}
		for key, value := range values {
			item[key] = value
		}
		breakdown = append(breakdown, item)
	}
	return breakdown
}

func parseRateLimitWindows(header http.Header) []rateLimitWindow {
	limits := splitHeaderNumbers(header.Get("X-RateLimit-Limit"))
	remaining := splitHeaderNumbers(header.Get("X-RateLimit-Remaining"))
	resets := splitHeaderNumbers(header.Get("X-RateLimit-Reset"))
	durations := parseRateLimitDurations(header.Get("X-RateLimit-Policy"))
	count := maxInt(len(limits), len(remaining), len(resets), len(durations))
	windows := make([]rateLimitWindow, 0, count)
	for i := 0; i < count; i++ {
		windows = append(windows, rateLimitWindow{
			Limit:     valueAt(limits, i),
			Remaining: valueAt(remaining, i),
			Reset:     valueAt(resets, i),
			Duration:  int(valueAt(durations, i)),
		})
	}
	return windows
}

func splitHeaderNumbers(value string) []float64 {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	numbers := make([]float64, 0, len(parts))
	for _, part := range parts {
		number, _ := strconv.ParseFloat(strings.TrimSpace(part), 64)
		numbers = append(numbers, number)
	}
	return numbers
}

func parseRateLimitDurations(policy string) []float64 {
	if strings.TrimSpace(policy) == "" {
		return nil
	}
	parts := strings.Split(policy, ",")
	durations := make([]float64, 0, len(parts))
	for _, part := range parts {
		duration := 0.0
		sections := strings.Split(strings.TrimSpace(part), ";")
		for _, section := range sections[1:] {
			section = strings.TrimSpace(section)
			if strings.HasPrefix(section, "w=") {
				duration, _ = strconv.ParseFloat(strings.TrimPrefix(section, "w="), 64)
				break
			}
		}
		durations = append(durations, duration)
	}
	return durations
}

func quotaWindowByLargestDuration(windows []rateLimitWindow) rateLimitWindow {
	if len(windows) == 0 {
		return rateLimitWindow{}
	}
	sorted := append([]rateLimitWindow(nil), windows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Duration == sorted[j].Duration {
			return sorted[i].Limit > sorted[j].Limit
		}
		return sorted[i].Duration > sorted[j].Duration
	})
	return sorted[0]
}

func rateLimitBreakdown(windows []rateLimitWindow) []map[string]interface{} {
	breakdown := make([]map[string]interface{}, 0, len(windows))
	for _, window := range windows {
		breakdown = append(breakdown, map[string]interface{}{
			"limit":     window.Limit,
			"remaining": window.Remaining,
			"reset":     window.Reset,
			"window":    window.Duration,
		})
	}
	return breakdown
}

func rateLimitRawText(header http.Header) string {
	keys := []string{"X-RateLimit-Limit", "X-RateLimit-Policy", "X-RateLimit-Remaining", "X-RateLimit-Reset"}
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := header.Get(key); value != "" {
			lines = append(lines, key+": "+value)
		}
	}
	return strings.Join(lines, "\n")
}

func valueAt(values []float64, index int) float64 {
	if index < 0 || index >= len(values) {
		return 0
	}
	return values[index]
}

func maxInt(values ...int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func objectArrayFromAny(value interface{}) []map[string]interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if mapped, ok := item.(map[string]interface{}); ok {
			result = append(result, mapped)
		}
	}
	return result
}

func stringFromAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func floatFromAny(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func truncateMessage(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
