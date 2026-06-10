package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
)

// QueryOfficialQuota queries the upstream provider's official quota/billing endpoint.
func QueryOfficialQuota(ctx context.Context, key model.APIKey, req model.ProviderKeyQuotaRequest) (model.ProviderKeyQuotaResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	switch key.ProviderName {
	case model.ProviderExa:
		return queryExaQuota(ctx, key, req)
	case model.ProviderYou:
		return queryYouQuota(ctx, key)
	case model.ProviderJina:
		return queryJinaQuota(ctx, key)
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
	endpoint := "https://admin-api.exa.ai/team-management/api-keys/" + url.PathEscape(apiKeyID) + "/usage"
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
	payload, err := doJSONQuotaRequest(httpReq)
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

func queryYouQuota(ctx context.Context, key model.APIKey) (model.ProviderKeyQuotaResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.you.com/v1/billing/account_balance", nil)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-API-Key", key.Value)
	payload, err := doJSONQuotaRequest(httpReq)
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

func queryJinaQuota(ctx context.Context, key model.APIKey) (model.ProviderKeyQuotaResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://r.jina.ai/", nil)
	if err != nil {
		return model.ProviderKeyQuotaResult{}, err
	}
	httpReq.Header.Set("Accept", "text/plain")
	httpReq.Header.Set("Authorization", "Bearer "+key.Value)
	client := &http.Client{Timeout: 30 * time.Second}
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

func doJSONQuotaRequest(req *http.Request) (map[string]interface{}, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("official quota query failed: status %d: %s", resp.StatusCode, truncateMessage(string(body), 500))
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode official quota response: %w", err)
	}
	return payload, nil
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
