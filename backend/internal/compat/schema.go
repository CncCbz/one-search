package compat

import "github.com/one-search/one-search/backend/internal/model"

type TavilySearchRequest struct {
	Query             string   `json:"query"`
	SearchDepth       string   `json:"search_depth,omitempty"`
	Topic             string   `json:"topic,omitempty"`
	Days              int      `json:"days,omitempty"`
	MaxResults        int      `json:"max_results,omitempty"`
	IncludeAnswer     bool     `json:"include_answer,omitempty"`
	IncludeRawContent bool     `json:"include_raw_content,omitempty"`
	IncludeImages     bool     `json:"include_images,omitempty"`
	IncludeDomains    []string `json:"include_domains,omitempty"`
	ExcludeDomains    []string `json:"exclude_domains,omitempty"`
	Providers         []string `json:"providers,omitempty"`
	Mode              string   `json:"mode,omitempty"`
	Cache             string   `json:"cache,omitempty"`
}

type TavilySearchResponse struct {
	Query        string         `json:"query"`
	Answer       string         `json:"answer,omitempty"`
	Results      []TavilyResult `json:"results"`
	ResponseTime float64        `json:"response_time"`
	RequestID    string         `json:"request_id"`
}

type TavilyResult struct {
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Content    string  `json:"content"`
	RawContent string  `json:"raw_content,omitempty"`
	Score      float64 `json:"score"`
}

type SerperSearchRequest struct {
	Q         string   `json:"q"`
	Num       int      `json:"num,omitempty"`
	Page      int      `json:"page,omitempty"`
	TBS       string   `json:"tbs,omitempty"`
	Gl        string   `json:"gl,omitempty"`
	Hl        string   `json:"hl,omitempty"`
	Providers []string `json:"providers,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	Cache     string   `json:"cache,omitempty"`
}

type SerperSearchResponse struct {
	SearchParameters map[string]interface{} `json:"searchParameters"`
	Organic          []SerperOrganicResult  `json:"organic"`
	Credits          int                    `json:"credits,omitempty"`
	RequestID        string                 `json:"requestId"`
}

type SerperOrganicResult struct {
	Title    string `json:"title"`
	Link     string `json:"link"`
	Snippet  string `json:"snippet"`
	Position int    `json:"position"`
}

type OpenAISearchRequest struct {
	Query     string   `json:"query,omitempty"`
	Input     string   `json:"input,omitempty"`
	Providers []string `json:"providers,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Cache     string   `json:"cache,omitempty"`
}

type OpenAISearchResponse struct {
	ID            string                   `json:"id"`
	Object        string                   `json:"object"`
	Status        string                   `json:"status"`
	SearchResults []model.SearchResult     `json:"search_results"`
	Output        []map[string]interface{} `json:"output"`
}
