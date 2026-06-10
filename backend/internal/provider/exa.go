package provider

import (
	"context"
	"net/http"

	"github.com/one-search/one-search/backend/internal/model"
)

type ExaProvider struct {
	*HTTPProvider
}

func NewExaProvider(cfg Config) *ExaProvider {
	cfg.Name = model.ProviderExa
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.exa.ai"
	}
	return &ExaProvider{HTTPProvider: NewHTTPProvider(cfg)}
}

func (p *ExaProvider) Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	body := map[string]interface{}{
		"query":      req.Query,
		"numResults": limit,
		"type":       "neural",
		"contents": map[string]interface{}{
			"text":       true,
			"highlights": true,
		},
	}
	request, err := p.newJSONRequest(ctx, http.MethodPost, "/search", body)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	request.Header.Set("Authorization", "Bearer "+key.Value)
	response, err := p.client.Do(request)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	payload, err := p.decodeResponse(response)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	results := normalizeExaResults(payload, req.IncludeRaw)
	return model.ProviderResponse{Results: results, Usage: usageMeasurements(model.ProviderExa, payload), Raw: payload}, nil
}

func normalizeExaResults(payload map[string]interface{}, includeRaw bool) []model.SearchResult {
	items := resultArray(payload, "results")
	results := make([]model.SearchResult, 0, len(items))
	for _, rawItem := range items {
		item := mapFromInterface(rawItem)
		if item == nil {
			continue
		}
		url := stringValue(item, "url")
		if url == "" {
			continue
		}
		snippet := firstStringFromArray(item, "highlights")
		if snippet == "" {
			snippet = stringValue(item, "text", "summary")
		}
		result := model.SearchResult{
			Title:       stringValue(item, "title"),
			URL:         url,
			Snippet:     snippet,
			Content:     stringValue(item, "text"),
			Provider:    model.ProviderExa,
			Providers:   []string{model.ProviderExa},
			Score:       floatValue(item, "score"),
			PublishedAt: parseTimeValue(stringValue(item, "publishedDate", "published_at")),
		}
		if includeRaw {
			result.Raw = item
		}
		results = append(results, result)
	}
	return results
}
