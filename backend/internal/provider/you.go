package provider

import (
	"context"
	"net/url"
	"strconv"

	"github.com/one-search/one-search/backend/internal/model"
)

type YouProvider struct {
	*HTTPProvider
}

func NewYouProvider(cfg Config) *YouProvider {
	cfg.Name = model.ProviderYou
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://ydc-index.io"
	}
	return &YouProvider{HTTPProvider: NewHTTPProvider(cfg)}
}

func (p *YouProvider) Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("query", req.Query)
	params.Set("count", strconv.Itoa(limit))
	request, err := p.newGETRequest(ctx, "/v1/search", params)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	request.Header.Set("X-API-Key", key.Value)
	response, err := p.client.Do(request)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	payload, err := p.decodeResponse(response)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	results := normalizeYouResults(payload, req.IncludeRaw)
	return model.ProviderResponse{Results: results, Raw: payload}, nil
}

func normalizeYouResults(payload map[string]interface{}, includeRaw bool) []model.SearchResult {
	items := resultArray(payload, "hits", "organic")
	if nested, ok := payload["results"].(map[string]interface{}); ok {
		items = append(items, resultArray(nested, "web")...)
		items = append(items, resultArray(nested, "news")...)
	} else {
		items = append(items, resultArray(payload, "results")...)
	}
	if web, ok := payload["web"].(map[string]interface{}); ok && len(items) == 0 {
		items = resultArray(web, "results", "hits")
	}
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
		snippet := stringValue(item, "snippet", "description")
		if snippet == "" {
			snippet = firstStringFromArray(item, "snippets")
		}
		content := stringValue(item, "content", "description", "snippet")
		if contents, ok := item["contents"].(map[string]interface{}); ok {
			content = stringValue(contents, "markdown", "html")
		}
		score := floatValue(item, "score")
		if score == 0 {
			score = 1 / float64(index+1)
		}
		result := model.SearchResult{
			Title:       stringValue(item, "title"),
			URL:         url,
			Snippet:     snippet,
			Content:     truncate(content, 4000),
			Provider:    model.ProviderYou,
			Providers:   []string{model.ProviderYou},
			Score:       score,
			PublishedAt: parseTimeValue(stringValue(item, "date", "publishedDate", "published_at", "page_age")),
		}
		if includeRaw {
			result.Raw = item
		}
		results = append(results, result)
	}
	return results
}
