package provider

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/one-search/one-search/backend/internal/model"
)

type BraveProvider struct {
	*HTTPProvider
}

func NewBraveProvider(cfg Config) *BraveProvider {
	cfg.Name = model.ProviderBrave
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.search.brave.com/res/v1"
	}
	return &BraveProvider{HTTPProvider: NewHTTPProvider(cfg)}
}

func (p *BraveProvider) Search(ctx context.Context, req model.SearchRequest, key model.APIKey) (model.ProviderResponse, error) {
	limit := requestLimit(req.Limit, 10, 20)
	params := url.Values{}
	params.Set("q", req.Query)
	params.Set("count", strconv.Itoa(limit))
	if freshness := braveFreshness(req); freshness != "" {
		params.Set("freshness", freshness)
	}
	if country := optionString(req.Options, "country"); country != "" {
		params.Set("country", country)
	}
	if lang := optionString(req.Options, "search_lang", "searchLang", "hl", "language"); lang != "" {
		params.Set("search_lang", lang)
	}
	if uiLang := optionString(req.Options, "ui_lang", "uiLang", "locale"); uiLang != "" {
		params.Set("ui_lang", uiLang)
	}
	if safesearch := optionString(req.Options, "safesearch", "safe_search", "safeSearch"); safesearch != "" {
		params.Set("safesearch", safesearch)
	}
	if offset := braveOffset(req, limit); offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	if req.IncludeRaw {
		params.Set("extra_snippets", "true")
	}
	request, err := p.newGETRequest(ctx, "/web/search", params)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	request.Header.Set("X-Subscription-Token", key.Value)
	response, err := p.client.Do(request)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	payload, err := p.decodeResponse(response)
	if err != nil {
		return model.ProviderResponse{}, err
	}
	results := normalizeBraveResults(payload, req.IncludeRaw)
	return model.ProviderResponse{Results: results, Usage: usageMeasurements(model.ProviderBrave, payload), Raw: payload}, nil
}

func braveFreshness(req model.SearchRequest) string {
	if value := optionString(req.Options, "freshness"); value != "" {
		return value
	}
	switch strings.ToLower(strings.TrimSpace(req.Freshness)) {
	case "day", "d", "qdr:d", "pd":
		return "pd"
	case "week", "w", "qdr:w", "pw":
		return "pw"
	case "month", "m", "qdr:m", "pm":
		return "pm"
	case "year", "y", "qdr:y", "py":
		return "py"
	default:
		return strings.TrimSpace(req.Freshness)
	}
}

func braveOffset(req model.SearchRequest, limit int) int {
	if offset := optionInt(req.Options, "offset"); offset > 0 {
		return offset
	}
	if page := optionInt(req.Options, "page"); page > 1 {
		return page - 1
	}
	if limit <= 0 {
		return 0
	}
	return 0
}

func normalizeBraveResults(payload map[string]interface{}, includeRaw bool) []model.SearchResult {
	web := mapFromInterface(payload["web"])
	if web == nil {
		return nil
	}
	items := resultArray(web, "results")
	results := make([]model.SearchResult, 0, len(items))
	for index, rawItem := range items {
		item := mapFromInterface(rawItem)
		if item == nil {
			continue
		}
		url := stringValue(item, "url")
		if url == "" {
			continue
		}
		snippet := stringValue(item, "description", "snippet")
		extraSnippets := stringArrayValue(item, "extra_snippets")
		content := snippet
		if len(extraSnippets) > 0 {
			content = strings.Join(append([]string{snippet}, extraSnippets...), "\n")
		}
		result := model.SearchResult{
			Title:       stringValue(item, "title"),
			URL:         url,
			Snippet:     truncate(snippet, 1000),
			Content:     truncate(content, 4000),
			Provider:    model.ProviderBrave,
			Providers:   []string{model.ProviderBrave},
			Score:       1 / float64(index+1),
			PublishedAt: parseTimeValue(stringValue(item, "age", "page_age", "published", "published_at", "date")),
		}
		if includeRaw {
			result.Raw = item
		}
		results = append(results, result)
	}
	return results
}
