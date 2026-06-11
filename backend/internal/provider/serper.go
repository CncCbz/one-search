package provider

import (
	"context"
	"net/http"

	"github.com/one-search/one-search/backend/internal/model"
)

type SerperProvider struct {
	*HTTPProvider
}

func NewSerperProvider(cfg Config) *SerperProvider {
	cfg.Name = model.ProviderSerper
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://google.serper.dev"
	}
	return &SerperProvider{HTTPProvider: NewHTTPProvider(cfg)}
}

func (p *SerperProvider) Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error) {
	body := map[string]interface{}{
		"q":   req.Query,
		"num": requestLimit(req.Limit, 10, 100),
	}
	if page := optionInt(req.Options, "page"); page > 0 {
		body["page"] = page
	}
	if tbs := optionString(req.Options, "tbs"); tbs != "" {
		body["tbs"] = tbs
	} else if req.Freshness != "" {
		body["tbs"] = req.Freshness
	}
	if gl := optionString(req.Options, "gl", "country"); gl != "" {
		body["gl"] = gl
	}
	if hl := optionString(req.Options, "hl", "locale", "language"); hl != "" {
		body["hl"] = hl
	}
	if location := optionString(req.Options, "location"); location != "" {
		body["location"] = location
	}
	request, err := p.newJSONRequest(ctx, http.MethodPost, "/search", body)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	request.Header.Set("X-API-KEY", key.Value)
	response, err := p.client.Do(request)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	payload, err := p.decodeResponse(response)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	results := normalizeSerperResults(payload, req.IncludeRaw)
	return model.ProviderResponse{Results: results, Usage: usageMeasurements(model.ProviderSerper, payload), Raw: payload}, nil
}

func normalizeSerperResults(payload map[string]interface{}, includeRaw bool) []model.SearchResult {
	items := append(resultArray(payload, "organic"), resultArray(payload, "news")...)
	results := make([]model.SearchResult, 0, len(items))
	for index, rawItem := range items {
		item := mapFromInterface(rawItem)
		if item == nil {
			continue
		}
		url := stringValue(item, "link", "url")
		if url == "" {
			continue
		}
		position := floatValue(item, "position")
		score := 1 / float64(index+1)
		if position > 0 {
			score = 1 / position
		}
		result := model.SearchResult{
			Title:       stringValue(item, "title"),
			URL:         url,
			Snippet:     truncate(stringValue(item, "snippet", "description"), 1000),
			Content:     truncate(stringValue(item, "snippet", "description"), 4000),
			Provider:    model.ProviderSerper,
			Providers:   []string{model.ProviderSerper},
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
