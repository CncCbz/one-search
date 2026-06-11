package provider

import (
	"context"
	"net/http"
	"strings"

	"github.com/one-search/one-search/backend/internal/model"
)

type FirecrawlProvider struct {
	*HTTPProvider
}

func NewFirecrawlProvider(cfg Config) *FirecrawlProvider {
	cfg.Name = model.ProviderFirecrawl
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.firecrawl.dev"
	}
	return &FirecrawlProvider{HTTPProvider: NewHTTPProvider(cfg)}
}

func (p *FirecrawlProvider) Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error) {
	body := map[string]interface{}{
		"query":   req.Query,
		"limit":   requestLimit(req.Limit, 10, 100),
		"sources": []string{"web"},
	}
	if tbs := firecrawlTBS(req); tbs != "" {
		body["tbs"] = tbs
	}
	if country := optionString(req.Options, "country"); country != "" {
		body["country"] = country
	}
	if location := optionString(req.Options, "location"); location != "" {
		body["location"] = location
	}
	if includeDomains := optionStringSlice(req.Options, "include_domains", "includeDomains"); len(includeDomains) > 0 {
		body["includeDomains"] = includeDomains
	}
	if excludeDomains := optionStringSlice(req.Options, "exclude_domains", "excludeDomains"); len(excludeDomains) > 0 {
		body["excludeDomains"] = excludeDomains
	}
	if timeout := optionInt(req.Options, "timeout"); timeout > 0 {
		body["timeout"] = timeout
	}
	if req.IncludeRaw {
		body["scrapeOptions"] = map[string]interface{}{"formats": []string{"markdown"}}
	}
	request, err := p.newJSONRequest(ctx, http.MethodPost, "/v2/search", body)
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
	results := normalizeFirecrawlResults(payload, req.IncludeRaw)
	return model.ProviderResponse{Results: results, Usage: usageMeasurements(model.ProviderFirecrawl, payload), Raw: payload}, nil
}

func firecrawlTBS(req model.SearchRequest) string {
	if value := optionString(req.Options, "tbs"); value != "" {
		return value
	}
	switch strings.ToLower(strings.TrimSpace(req.Freshness)) {
	case "hour", "qdr:h":
		return "qdr:h"
	case "day", "d", "qdr:d", "pd":
		return "qdr:d"
	case "week", "w", "qdr:w", "pw":
		return "qdr:w"
	case "month", "m", "qdr:m", "pm":
		return "qdr:m"
	case "year", "y", "qdr:y", "py":
		return "qdr:y"
	default:
		return req.Freshness
	}
}

func normalizeFirecrawlResults(payload map[string]interface{}, includeRaw bool) []model.SearchResult {
	items := resultArray(payload, "data")
	if data := mapFromInterface(payload["data"]); data != nil {
		items = append(resultArray(data, "web"), resultArray(data, "news")...)
	}
	results := make([]model.SearchResult, 0, len(items))
	for index, rawItem := range items {
		item := mapFromInterface(rawItem)
		if item == nil {
			continue
		}
		metadata := mapFromInterface(item["metadata"])
		url := stringValue(item, "url")
		if url == "" && metadata != nil {
			url = stringValue(metadata, "sourceURL", "url")
		}
		if url == "" {
			continue
		}
		title := stringValue(item, "title")
		if title == "" && metadata != nil {
			title = stringValue(metadata, "title")
		}
		snippet := stringValue(item, "description", "snippet")
		if snippet == "" && metadata != nil {
			snippet = stringValue(metadata, "description")
		}
		content := stringValue(item, "markdown", "html", "rawHtml", "description", "snippet")
		position := floatValue(item, "position")
		score := 1 / float64(index+1)
		if position > 0 {
			score = 1 / position
		}
		result := model.SearchResult{
			Title:       title,
			URL:         url,
			Snippet:     truncate(snippet, 1000),
			Content:     truncate(content, 4000),
			Provider:    model.ProviderFirecrawl,
			Providers:   []string{model.ProviderFirecrawl},
			Score:       score,
			PublishedAt: parseTimeValue(stringValue(item, "date", "publishedDate", "published_at")),
		}
		if includeRaw {
			result.Raw = item
		}
		results = append(results, result)
	}
	return results
}
