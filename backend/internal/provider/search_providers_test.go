package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/one-search/one-search/backend/internal/model"
)

func TestTavilyProviderSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/search" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tavily-key" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["query"] != "golang" || body["max_results"] != float64(20) || body["search_depth"] != "advanced" || body["topic"] != "news" {
			t.Fatalf("unexpected request body: %#v", body)
		}
		writeJSON(t, w, map[string]interface{}{
			"usage": map[string]interface{}{"credits": 2},
			"results": []map[string]interface{}{
				{"title": "Tavily result", "url": "https://example.com/tavily", "content": "summary", "raw_content": "full content", "score": 0.7, "published_date": "2024-01-02"},
			},
		})
	}))
	defer server.Close()

	provider := NewTavilyProvider(Config{BaseURL: server.URL})
	response, err := provider.Search(context.Background(), model.SearchRequest{
		Query:      "golang",
		Limit:      50,
		IncludeRaw: true,
		Options: map[string]interface{}{
			"search_depth":    "advanced",
			"topic":           "news",
			"include_domains": []interface{}{"example.com"},
		},
	}, model.APIKey{Value: "tavily-key"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].Provider != model.ProviderTavily || response.Results[0].Raw == nil {
		t.Fatalf("unexpected results: %#v", response.Results)
	}
	if len(response.Usage) != 1 || response.Usage[0].Unit != "credits" || response.Usage[0].Quantity != 2 {
		t.Fatalf("unexpected usage: %#v", response.Usage)
	}
}

func TestFirecrawlProviderSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/search" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer firecrawl-key" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["query"] != "scraping" || body["limit"] != float64(100) || body["tbs"] != "qdr:w" {
			t.Fatalf("unexpected request body: %#v", body)
		}
		scrapeOptions, _ := body["scrapeOptions"].(map[string]interface{})
		formats, _ := scrapeOptions["formats"].([]interface{})
		if len(formats) != 1 || formats[0] != "markdown" {
			t.Fatalf("unexpected scrapeOptions: %#v", scrapeOptions)
		}
		writeJSON(t, w, map[string]interface{}{
			"success":     true,
			"creditsUsed": 3,
			"data": []map[string]interface{}{
				{"title": "Firecrawl result", "url": "https://example.com/firecrawl", "description": "desc", "markdown": "full markdown", "metadata": map[string]interface{}{"statusCode": 200}},
				{"title": "Firecrawl news", "url": "https://example.com/firecrawl-news", "snippet": "news desc", "date": "2024-02-03", "position": 2},
			},
		})
	}))
	defer server.Close()

	provider := NewFirecrawlProvider(Config{BaseURL: server.URL})
	response, err := provider.Search(context.Background(), model.SearchRequest{Query: "scraping", Limit: 150, IncludeRaw: true, Freshness: "week"}, model.APIKey{Value: "firecrawl-key"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(response.Results) != 2 || response.Results[0].Provider != model.ProviderFirecrawl || response.Results[0].Content != "full markdown" {
		t.Fatalf("unexpected results: %#v", response.Results)
	}
	if response.Results[1].PublishedAt == nil {
		t.Fatalf("expected published date: %#v", response.Results[1])
	}
	if len(response.Usage) != 1 || response.Usage[0].Quantity != 3 {
		t.Fatalf("unexpected usage: %#v", response.Usage)
	}
}

func TestSerperProviderSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/search" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-API-KEY"); got != "serper-key" {
			t.Fatalf("X-API-KEY = %q", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["q"] != "serp" || body["num"] != float64(10) || body["gl"] != "us" || body["hl"] != "en" || body["page"] != float64(2) {
			t.Fatalf("unexpected request body: %#v", body)
		}
		writeJSON(t, w, map[string]interface{}{
			"credits": 1,
			"organic": []map[string]interface{}{
				{"title": "Serper result", "link": "https://example.com/serper", "snippet": "serper desc", "date": "2024-03-04", "position": 4},
			},
		})
	}))
	defer server.Close()

	provider := NewSerperProvider(Config{BaseURL: server.URL})
	response, err := provider.Search(context.Background(), model.SearchRequest{
		Query: "serp",
		Options: map[string]interface{}{
			"gl":   "us",
			"hl":   "en",
			"page": 2,
		},
	}, model.APIKey{Value: "serper-key"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].Provider != model.ProviderSerper || response.Results[0].Score != 0.25 {
		t.Fatalf("unexpected results: %#v", response.Results)
	}
	if len(response.Usage) != 1 || response.Usage[0].Unit != "credits" || response.Usage[0].Quantity != 1 {
		t.Fatalf("unexpected usage: %#v", response.Usage)
	}
}

func TestBraveProviderSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/web/search" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Subscription-Token"); got != "brave-key" {
			t.Fatalf("X-Subscription-Token = %q", got)
		}
		query := r.URL.Query()
		if query.Get("q") != "privacy" || query.Get("count") != "20" || query.Get("freshness") != "pw" || query.Get("extra_snippets") != "true" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		writeJSON(t, w, map[string]interface{}{
			"web": map[string]interface{}{
				"results": []map[string]interface{}{
					{"title": "Brave result", "url": "https://example.com/brave", "description": "brave desc", "extra_snippets": []string{"more context"}, "age": "2024-04-05"},
				},
			},
		})
	}))
	defer server.Close()

	provider := NewBraveProvider(Config{BaseURL: server.URL})
	response, err := provider.Search(context.Background(), model.SearchRequest{Query: "privacy", Limit: 50, IncludeRaw: true, Freshness: "week"}, model.APIKey{Value: "brave-key"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].Provider != model.ProviderBrave || !strings.Contains(response.Results[0].Content, "more context") || response.Results[0].Raw == nil {
		t.Fatalf("unexpected results: %#v", response.Results)
	}
	if len(response.Usage) != 0 {
		t.Fatalf("unexpected usage: %#v", response.Usage)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
