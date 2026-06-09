package provider

import (
	"context"
	"net/url"

	"github.com/one-search/one-search/backend/internal/model"
)

type JinaProvider struct {
	*HTTPProvider
}

func NewJinaProvider(cfg Config) *JinaProvider {
	cfg.Name = model.ProviderJina
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://s.jina.ai"
	}
	return &JinaProvider{HTTPProvider: NewHTTPProvider(cfg)}
}

func (p *JinaProvider) Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error) {
	request, err := p.newGETRequest(ctx, "/"+url.PathEscape(req.Query), nil)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	request.Header.Set("Accept", "application/json")
	if key.Value != "" {
		request.Header.Set("Authorization", "Bearer "+key.Value)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	payload, err := p.decodeResponse(response)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	results := normalizeJinaResults(payload, req.IncludeRaw)
	return model.ProviderResponse{Results: results, Raw: payload}, nil
}

func normalizeJinaResults(payload map[string]interface{}, includeRaw bool) []model.SearchResult {
	items := resultArray(payload, "data", "results")
	results := make([]model.SearchResult, 0, len(items))
	for index, rawItem := range items {
		item := mapFromInterface(rawItem)
		if item == nil {
			continue
		}
		url := stringValue(item, "url", "link")
		if url == "" {
			continue
		}
		score := floatValue(item, "score")
		if score == 0 {
			score = 1 / float64(index+1)
		}
		result := model.SearchResult{
			Title:       stringValue(item, "title"),
			URL:         url,
			Snippet:     truncate(stringValue(item, "description", "snippet", "content"), 1000),
			Content:     truncate(stringValue(item, "content", "text", "description"), 4000),
			Provider:    model.ProviderJina,
			Providers:   []string{model.ProviderJina},
			Score:       score,
			PublishedAt: parseTimeValue(stringValue(item, "published", "published_at", "date")),
		}
		if includeRaw {
			result.Raw = item
		}
		results = append(results, result)
	}
	return results
}
