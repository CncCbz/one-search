package compat

import (
	"fmt"
	"strings"

	"github.com/one-search/one-search/backend/internal/model"
)

func TavilyToNative(req TavilySearchRequest) model.SearchRequest {
	options := map[string]interface{}{}
	if req.SearchDepth != "" {
		options["search_depth"] = req.SearchDepth
	}
	if req.Topic != "" {
		options["topic"] = req.Topic
	}
	if req.Days > 0 {
		options["days"] = req.Days
	}
	if len(req.IncludeDomains) > 0 {
		options["include_domains"] = req.IncludeDomains
	}
	if len(req.ExcludeDomains) > 0 {
		options["exclude_domains"] = req.ExcludeDomains
	}
	return model.SearchRequest{
		Query:        req.Query,
		Providers:    req.Providers,
		Mode:         model.SearchMode(req.Mode),
		Limit:        req.MaxResults,
		IncludeRaw:   req.IncludeRawContent,
		Cache:        model.CachePolicy(req.Cache),
		CompatFormat: model.CompatFormatTavily,
		Options:      options,
	}
}

func TavilyFromNative(query string, response model.SearchResponse) TavilySearchResponse {
	results := make([]TavilyResult, 0, len(response.Results))
	for _, item := range response.Results {
		results = append(results, TavilyResult{
			Title:      item.Title,
			URL:        item.URL,
			Content:    firstNonEmpty(item.Snippet, item.Content),
			RawContent: item.Content,
			Score:      item.Score,
		})
	}
	return TavilySearchResponse{
		Query:        query,
		Results:      results,
		ResponseTime: float64(response.Meta.LatencyMS) / 1000,
		RequestID:    response.Meta.RequestID,
	}
}

func SerperToNative(req SerperSearchRequest) model.SearchRequest {
	options := map[string]interface{}{}
	if req.Page > 0 {
		options["page"] = req.Page
	}
	if req.TBS != "" {
		options["tbs"] = req.TBS
	}
	if req.Gl != "" {
		options["gl"] = req.Gl
	}
	if req.Hl != "" {
		options["hl"] = req.Hl
	}
	return model.SearchRequest{
		Query:        req.Q,
		Providers:    req.Providers,
		Mode:         model.SearchMode(req.Mode),
		Limit:        req.Num,
		Freshness:    req.TBS,
		Cache:        model.CachePolicy(req.Cache),
		CompatFormat: model.CompatFormatSerper,
		Options:      options,
	}
}

func SerperFromNative(req SerperSearchRequest, response model.SearchResponse) SerperSearchResponse {
	organic := make([]SerperOrganicResult, 0, len(response.Results))
	for index, item := range response.Results {
		organic = append(organic, SerperOrganicResult{
			Title:    item.Title,
			Link:     item.URL,
			Snippet:  item.Snippet,
			Position: index + 1,
		})
	}
	return SerperSearchResponse{
		SearchParameters: map[string]interface{}{"q": req.Q, "num": req.Num, "type": "search"},
		Organic:          organic,
		Credits:          len(response.Meta.ProvidersQueried),
		RequestID:        response.Meta.RequestID,
	}
}

func OpenAIToNative(req OpenAISearchRequest) model.SearchRequest {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		query = strings.TrimSpace(req.Input)
	}
	return model.SearchRequest{
		Query:        query,
		Providers:    req.Providers,
		Mode:         model.SearchMode(req.Mode),
		Limit:        req.Limit,
		Cache:        model.CachePolicy(req.Cache),
		CompatFormat: model.CompatFormatOpenAI,
	}
}

func OpenAIFromNative(response model.SearchResponse) OpenAISearchResponse {
	lines := make([]string, 0, len(response.Results))
	for index, item := range response.Results {
		lines = append(lines, fmt.Sprintf("%d. %s - %s\n%s", index+1, item.Title, item.URL, firstNonEmpty(item.Snippet, item.Content)))
	}
	return OpenAISearchResponse{
		ID:            response.Meta.RequestID,
		Object:        "response",
		Status:        "completed",
		SearchResults: response.Results,
		Output: []map[string]interface{}{
			{"type": "web_search_call", "status": "completed"},
			{"type": "message", "content": []map[string]string{{"type": "output_text", "text": strings.Join(lines, "\n\n")}}},
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
