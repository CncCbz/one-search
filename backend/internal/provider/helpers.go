package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/one-search/one-search/backend/internal/model"
)

type Config struct {
	Name      string
	BaseURL   string
	UserAgent string
	Timeout   time.Duration
}

type HTTPProvider struct {
	name      string
	baseURL   string
	userAgent string
	client    *http.Client
}

func NewHTTPProvider(cfg Config) *HTTPProvider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &HTTPProvider{
		name:      cfg.Name,
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		userAgent: cfg.UserAgent,
		client:    &http.Client{Timeout: timeout},
	}
}

func (p *HTTPProvider) Name() string {
	return p.name
}

func (p *HTTPProvider) HealthCheck(ctx context.Context, key model.APIKey) error {
	if strings.TrimSpace(key.Value) == "" {
		return &Error{Type: ErrorTypeNoKey, Message: "empty api key"}
	}
	return nil
}

func (p *HTTPProvider) newJSONRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, p.baseURL+endpoint, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	if p.userAgent != "" {
		request.Header.Set("User-Agent", p.userAgent)
	}
	return request, nil
}

func (p *HTTPProvider) newGETRequest(ctx context.Context, endpoint string, params url.Values) (*http.Request, error) {
	requestURL := p.baseURL + endpoint
	if len(params) > 0 {
		requestURL += "?" + params.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if p.userAgent != "" {
		request.Header.Set("User-Agent", p.userAgent)
	}
	return request, nil
}

func (p *HTTPProvider) decodeResponse(response *http.Response) (map[string]interface{}, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, ClassifyHTTPError(response.StatusCode, string(body))
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &Error{Type: ErrorTypeInvalidResponse, StatusCode: response.StatusCode, Message: err.Error()}
	}
	return payload, nil
}

func stringValue(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case string:
				return typed
			case float64:
				return strings.TrimRight(strings.TrimRight(jsonNumber(typed), "0"), ".")
			}
		}
	}
	return ""
}

func floatValue(item map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case float64:
				return typed
			case int:
				return float64(typed)
			case string:
				if parsed, err := strconv.ParseFloat(typed, 64); err == nil {
					return parsed
				}
			}
		}
	}
	return 0
}

func firstStringFromArray(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		values, ok := item[key].([]interface{})
		if !ok || len(values) == 0 {
			continue
		}
		if first, ok := values[0].(string); ok {
			return first
		}
	}
	return ""
}

func resultArray(payload map[string]interface{}, keys ...string) []interface{} {
	for _, key := range keys {
		if values, ok := payload[key].([]interface{}); ok {
			return values
		}
	}
	return nil
}

func mapFromInterface(value interface{}) map[string]interface{} {
	if item, ok := value.(map[string]interface{}); ok {
		return item
	}
	return nil
}

func parseTimeValue(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05Z"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func jsonNumber(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', -1, 64), "0"), ".")
}

func usageMeasurements(providerName string, payload map[string]interface{}) []model.UsageMeasurement {
	if payload == nil {
		return nil
	}
	metadata := map[string]interface{}{"provider": providerName}
	measurements := []model.UsageMeasurement{}
	if tokens := usageNumber(payload, "total_tokens", "tokens", "token_count", "usage.total_tokens", "usage.tokens"); tokens > 0 {
		measurements = append(measurements, model.UsageMeasurement{Unit: "tokens", Quantity: tokens, Metadata: metadata})
	}
	cost := usageNumber(payload, "cost_usd", "total_cost_usd", "usage.cost_usd", "usage.total_cost_usd")
	if cost == 0 {
		cost = usageNumber(payload, "cost", "total_cost", "usage.cost", "usage.total_cost")
	}
	if cost > 0 {
		measurements = append(measurements, model.UsageMeasurement{Unit: "usd", Quantity: cost, CostUSD: float64Pointer(cost), Metadata: metadata})
	}
	return measurements
}

func usageNumber(payload map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		value, ok := nestedValue(payload, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed
		case int:
			return float64(typed)
		case int64:
			return float64(typed)
		case json.Number:
			if parsed, err := typed.Float64(); err == nil {
				return parsed
			}
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func nestedValue(payload map[string]interface{}, dotted string) (interface{}, bool) {
	parts := strings.Split(dotted, ".")
	var current interface{} = payload
	for _, part := range parts {
		item, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = item[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func float64Pointer(value float64) *float64 {
	return &value
}

func truncate(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
