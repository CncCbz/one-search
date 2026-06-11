package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/one-search/one-search/backend/internal/model"
)

func TestMCPStreamableHTTPHandshakeAndListTools(t *testing.T) {
	h := &Handler{}
	r := chi.NewRouter()
	h.mountMCP(r, "/mcp")

	post := func(payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	initRec := post(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"initialize",
		"params":{
			"protocolVersion":"2025-06-18",
			"capabilities":{},
			"clientInfo":{"name":"test-client","version":"1.0.0"}
		}
	}`)
	if initRec.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, body = %s", initRec.Code, initRec.Body.String())
	}
	var initResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			Capabilities    struct {
				Tools map[string]interface{} `json:"tools"`
			} `json:"capabilities"`
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(initRec.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("decode initialize response: %v", err)
	}
	if initResp.JSONRPC != "2.0" || initResp.ID != 1 || initResp.Result.ProtocolVersion != "2025-06-18" {
		t.Fatalf("unexpected initialize response: %+v", initResp)
	}
	if initResp.Result.ServerInfo.Name != "one-search-relay" || initResp.Result.ServerInfo.Version == "" {
		t.Fatalf("unexpected serverInfo: %+v", initResp.Result.ServerInfo)
	}
	if initResp.Result.Capabilities.Tools == nil {
		t.Fatalf("tools capability missing: %+v", initResp.Result.Capabilities)
	}

	initializedRec := post(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if initializedRec.Code != http.StatusAccepted {
		t.Fatalf("initialized status = %d, body = %s", initializedRec.Code, initializedRec.Body.String())
	}

	toolsRec := post(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if toolsRec.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d, body = %s", toolsRec.Code, toolsRec.Body.String())
	}
	var toolsResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				InputSchema map[string]interface{} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(toolsRec.Body.Bytes(), &toolsResp); err != nil {
		t.Fatalf("decode tools/list response: %v", err)
	}
	if toolsResp.JSONRPC != "2.0" || toolsResp.ID != 2 || len(toolsResp.Result.Tools) != 1 {
		t.Fatalf("unexpected tools/list response: %+v", toolsResp)
	}
	tool := toolsResp.Result.Tools[0]
	if tool.Name != "search" || tool.Description == "" || tool.InputSchema["type"] != "object" {
		t.Fatalf("unexpected tool schema: %+v", tool)
	}
	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("tool properties missing: %+v", tool.InputSchema)
	}
	providers, ok := properties["providers"].(map[string]interface{})
	if !ok {
		t.Fatalf("providers schema missing: %+v", properties)
	}
	items, ok := providers["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("providers items schema missing: %+v", providers)
	}
	enumValues, ok := items["enum"].([]interface{})
	if !ok || len(enumValues) != len(model.DefaultProviders) {
		t.Fatalf("unexpected providers enum: %+v", items["enum"])
	}
	for index, provider := range model.DefaultProviders {
		if enumValues[index] != provider {
			t.Fatalf("providers enum[%d] = %v, want %s", index, enumValues[index], provider)
		}
	}
}

func TestMCPStreamableHTTPGetSSEIsNotOffered(t *testing.T) {
	h := &Handler{}
	r := chi.NewRouter()
	h.mountMCP(r, "/mcp")

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET SSE status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
