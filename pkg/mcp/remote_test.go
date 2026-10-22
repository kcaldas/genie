package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	genieconfig "github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/tools"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// Exercise Genie's public client and metadata adapters against an independent
// implementation, including both legal Streamable HTTP response forms.
func TestRemoteMCP(t *testing.T) {
	for _, jsonResponse := range []bool{false, true} {
		name := "sse"
		if jsonResponse {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			server := sdk.NewServer(&sdk.Implementation{Name: "fixture", Version: "1"}, nil)
			server.AddTool(&sdk.Tool{Name: "record", Title: "Record", Description: "Record a receipt", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"description": map[string]any{"type": "string"}}, "required": []string{"description"}}}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
				return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "recorded"}}, StructuredContent: []any{"receipt", true}}, nil
			})
			handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: jsonResponse})
			var mu sync.Mutex
			var methods []string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-token" {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				mu.Lock()
				methods = append(methods, r.Header.Get("Mcp-Method"))
				mu.Unlock()
				if r.Header.Get("Mcp-Method") == "tools/call" {
					require.Equal(t, "2026-07-28", r.Header.Get("MCP-Protocol-Version"))
					require.Equal(t, "record", r.Header.Get("Mcp-Name"))
					require.Empty(t, r.Header.Get("Mcp-Session-Id"))
				}
				handler.ServeHTTP(w, r)
			}))
			defer ts.Close()
			cfg := &Config{McpServers: map[string]ServerConfig{"accounts": {Type: "http", URL: ts.URL, Headers: map[string]string{"Authorization": "Bearer test-token"}, DisplayName: "Accounting", SignalText: tools.LocalizedText{Default: "Working"}, Tools: map[string]ToolDisplayConfig{"record": {DisplayName: "Record receipt", SignalText: tools.LocalizedText{Default: "Recording", Locales: map[string]string{"pt-BR": "Registrando"}}}}}}}
			client := NewClient(cfg, genieconfig.NewConfigManager())
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, client.ConnectToServers(ctx))
			require.Empty(t, client.ServerErrors())
			discovered := client.GetTools()
			require.Len(t, discovered, 1)
			require.Equal(t, "record", discovered[0].Declaration().Name)
			result, err := discovered[0].Handler()(ctx, map[string]any{"description": "Paper"})
			require.NoError(t, err)
			require.Equal(t, []any{"receipt", true}, result.Details["structuredContent"])
			meta := client.ToolsMeta()["record"]
			require.Equal(t, "Accounting", meta.ServerDisplayName)
			require.Equal(t, "Record receipt", meta.DisplayName)
			require.Equal(t, "Registrando", meta.SignalText.Resolve("pt-BR"))
			mu.Lock()
			defer mu.Unlock()
			require.Contains(t, methods, "server/discover")
			require.NotContains(t, methods, "initialize")
		})
	}
}

func TestRemoteCancellationDoesNotReplayTool(t *testing.T) {
	server := sdk.NewServer(&sdk.Implementation{Name: "fixture", Version: "1"}, nil)
	started := make(chan struct{})
	stopped := make(chan struct{})
	server.AddTool(&sdk.Tool{Name: "wait", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		close(started)
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		close(stopped)
		return nil, ctx.Err()
	})
	ts := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true, PropagateRequestCancellation: true}))
	defer ts.Close()
	client := NewClient(&Config{McpServers: map[string]ServerConfig{"remote": {Type: "http", URL: ts.URL}}}, genieconfig.NewConfigManager())
	defer client.Close()
	require.NoError(t, client.ConnectToServers(context.Background()))
	require.Empty(t, client.ServerErrors())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := client.CallTool(ctx, "wait", nil); done <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("tool did not start")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("call did not cancel")
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not observe cancellation")
	}
}

func TestRemoteRejectsCrossOriginRedirect(t *testing.T) {
	var mu sync.Mutex
	leaked := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		leaked = true
		mu.Unlock()
		w.WriteHeader(500)
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	client := NewClient(&Config{McpServers: map[string]ServerConfig{"remote": {Type: "http", URL: source.URL, Headers: map[string]string{"X-Api-Key": "private"}}}}, genieconfig.NewConfigManager())
	defer client.Close()
	require.NoError(t, client.ConnectToServers(context.Background()))
	require.NotEmpty(t, client.ServerErrors())
	mu.Lock()
	defer mu.Unlock()
	require.False(t, leaked)
}

func TestRemotePaginationAndSchema(t *testing.T) {
	// Raw wire fixture verifies pagination and schema preservation independently
	// of SDK server-side schema normalization.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var request struct {
			ID     any            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		require.NoError(t, json.Unmarshal(raw, &request))
		var result any
		switch request.Method {
		case "server/discover":
			result = map[string]any{"resultType": "complete", "supportedVersions": []string{"2026-07-28"}, "serverInfo": map[string]any{"name": "fixture", "version": "1"}, "capabilities": map[string]any{"tools": map[string]any{}}}
		case "tools/list":
			result = map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{map[string]any{"name": "first", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}}}, "nextCursor": "page2"}
			if request.Params["cursor"] == "page2" {
				result = map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{map[string]any{"name": "second", "inputSchema": map[string]any{"type": "object", "$defs": map[string]any{"value": map[string]any{"type": []string{"string", "null"}}}, "properties": map[string]any{"value": map[string]any{"$ref": "#/$defs/value"}}, "additionalProperties": false}}}}
			}
		default:
			t.Errorf("unexpected method %s", request.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer ts.Close()
	client := NewClient(&Config{McpServers: map[string]ServerConfig{"remote": {Type: "http", URL: ts.URL}}}, genieconfig.NewConfigManager())
	defer client.Close()
	require.NoError(t, client.ConnectToServers(context.Background()))
	require.Empty(t, client.ServerErrors())
	require.Len(t, client.GetTools(), 2)
	// Inspect the complete schema through the catalog, as a progressive-discovery
	// consumer would, rather than round-tripping through an LLM's schema subset.
	raw, _ := json.Marshal(client.GetTools()[1].Declaration())
	require.True(t, strings.Contains(string(raw), "$defs"), string(raw))
}

func TestRemoteInputRequired(t *testing.T) {
	server := sdk.NewServer(&sdk.Implementation{Name: "fixture", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "confirm", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &sdk.CallToolResult{InputRequests: sdk.InputRequestMap{"confirmation": &sdk.ElicitParams{Message: "Continue?"}}, RequestState: "opaque-state"}, nil
		}
		if req.Params.RequestState != "opaque-state" {
			t.Error("lost request state")
		}
		response, ok := req.Params.InputResponses["confirmation"].(*sdk.ElicitResult)
		if !ok || response.Action != "accept" {
			t.Errorf("lost input response: %#v", response)
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "confirmed"}}}, nil
	})
	ts := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true}))
	defer ts.Close()
	cfg := &Config{McpServers: map[string]ServerConfig{"remote": {Type: "http", URL: ts.URL}}}
	calls := 0
	client := NewClientWithOptions(cfg, genieconfig.NewConfigManager(), Options{ClientOptions: func(string) *sdk.ClientOptions {
		return &sdk.ClientOptions{ElicitationHandler: func(ctx context.Context, req *sdk.ElicitRequest) (*sdk.ElicitResult, error) {
			calls++
			return &sdk.ElicitResult{Action: "accept"}, nil
		}}
	}})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.ConnectToServers(ctx))
	require.Empty(t, client.ServerErrors())
	result, err := client.CallTool(ctx, "confirm", nil)
	require.NoError(t, err)
	require.Equal(t, "confirmed", result.Content[0].Text)
	require.Equal(t, 1, calls)
}

func TestRemoteResourcesPromptsAndCatalogUpdates(t *testing.T) {
	server := sdk.NewServer(&sdk.Implementation{Name: "fixture", Version: "1"}, nil)
	handler := func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "done"}}}, nil
	}
	server.AddTool(&sdk.Tool{Name: "old", InputSchema: map[string]any{"type": "object"}}, handler)
	server.AddResource(&sdk.Resource{URI: "fixture://receipt", Name: "receipt", MIMEType: "application/pdf"}, func(context.Context, *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{{URI: "fixture://receipt", MIMEType: "application/pdf", Blob: []byte("%PDF-test")}}}, nil
	})
	server.AddPrompt(&sdk.Prompt{Name: "summarize"}, func(context.Context, *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
		return &sdk.GetPromptResult{Messages: []*sdk.PromptMessage{{Role: "user", Content: &sdk.TextContent{Text: "Summarize the receipt"}}}}, nil
	})
	ts := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true}))
	defer ts.Close()
	changed := make(chan string, 10)
	client := NewClientWithOptions(&Config{McpServers: map[string]ServerConfig{"remote": {Type: "http", URL: ts.URL}}}, genieconfig.NewConfigManager(), Options{ToolsChanged: func(server string) { changed <- server }})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	require.NoError(t, client.ConnectToServers(ctx))
	require.Empty(t, client.ServerErrors())
	old := client.GetTools()[0]
	session, err := client.Session("remote")
	require.NoError(t, err)
	resources, err := session.ListResources(ctx, nil)
	require.NoError(t, err)
	require.Len(t, resources.Resources, 1)
	resource, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: "fixture://receipt"})
	require.NoError(t, err)
	require.Equal(t, []byte("%PDF-test"), resource.Contents[0].Blob)
	prompts, err := session.ListPrompts(ctx, nil)
	require.NoError(t, err)
	require.Len(t, prompts.Prompts, 1)
	prompt, err := session.GetPrompt(ctx, &sdk.GetPromptParams{Name: "summarize"})
	require.NoError(t, err)
	require.Len(t, prompt.Messages, 1)
	server.RemoveTools("old")
	server.AddTool(&sdk.Tool{Name: "new", InputSchema: map[string]any{"type": "object"}}, handler)
	require.Eventually(t, func() bool {
		found := client.GetTools()
		return len(found) == 1 && found[0].Declaration().Name == "new"
	}, 3*time.Second, 10*time.Millisecond)
	select {
	case name := <-changed:
		require.Equal(t, "remote", name)
	case <-ctx.Done():
		t.Fatal("missing tools-changed callback")
	}
	_, err = old.Handler()(ctx, nil)
	require.ErrorContains(t, err, "no longer available")
	_, err = client.CallTool(ctx, "new", nil)
	require.NoError(t, err)
}

func TestRemoteToolPrefixesPreserveMetadataAndRouting(t *testing.T) {
	server := sdk.NewServer(&sdk.Implementation{Name: "fixture", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "echo", Title: "Echo", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "ok"}}}, nil
	})
	ts := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true}))
	defer ts.Close()
	cfg := &Config{McpServers: map[string]ServerConfig{
		"first":  {Type: "http", URL: ts.URL, ToolPrefix: "one_", Tools: map[string]ToolDisplayConfig{"echo": {DisplayName: "First echo", SignalText: tools.LocalizedText{Locales: map[string]string{"en": "Echoing", "pt-BR": "Repetindo"}}}}},
		"second": {Type: "http", URL: ts.URL, ToolPrefix: "two_"},
	}}
	client := NewClient(cfg, genieconfig.NewConfigManager())
	defer client.Close()
	require.NoError(t, client.ConnectToServers(context.Background()))
	require.Empty(t, client.ServerErrors())
	require.Len(t, client.GetTools(), 2)
	require.Equal(t, "First echo", client.ToolsMeta()["one_echo"].DisplayName)
	require.Equal(t, "Repetindo", client.ToolsMeta()["one_echo"].SignalText.Resolve("pt"))
	_, err := client.CallTool(context.Background(), "two_echo", nil)
	require.NoError(t, err)
	ambiguous := NewClient(&Config{McpServers: map[string]ServerConfig{"a": {Type: "http", URL: ts.URL}, "b": {Type: "http", URL: ts.URL}}}, genieconfig.NewConfigManager())
	defer ambiguous.Close()
	require.NoError(t, ambiguous.ConnectToServers(context.Background()))
	require.Empty(t, ambiguous.GetTools())
	_, err = ambiguous.CallTool(context.Background(), "echo", nil)
	require.ErrorContains(t, err, "ambiguous")
}

func TestLegacyRemoteTransports(t *testing.T) {
	for _, transport := range []string{"http", "sse"} {
		t.Run(transport, func(t *testing.T) {
			version := "2025-11-25"
			if transport == "sse" {
				version = "2024-11-05"
			}
			server := sdk.NewServer(&sdk.Implementation{Name: "legacy", Version: "1"}, &sdk.ServerOptions{SupportedProtocolVersions: []string{version}})
			server.AddTool(&sdk.Tool{Name: "echo", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
				return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "legacy"}}}, nil
			})
			var handler http.Handler = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, nil)
			if transport == "sse" {
				handler = sdk.NewSSEHandler(func(*http.Request) *sdk.Server { return server }, nil)
			}
			ts := httptest.NewServer(handler)
			defer ts.Close()
			client := NewClient(&Config{McpServers: map[string]ServerConfig{"legacy": {Type: transport, URL: ts.URL}}}, genieconfig.NewConfigManager())
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, client.ConnectToServers(ctx))
			require.Empty(t, client.ServerErrors())
			session, err := client.Session("legacy")
			require.NoError(t, err)
			require.Equal(t, version, session.InitializeResult().ProtocolVersion)
			result, err := client.CallTool(ctx, "echo", nil)
			require.NoError(t, err)
			require.Equal(t, "legacy", result.Content[0].Text)
		})
	}
}

func TestRemoteRejectsCyclicPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     any
			Method string
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		var result any
		switch request.Method {
		case "server/discover":
			result = map[string]any{"resultType": "complete", "supportedVersions": []string{"2026-07-28"}, "serverInfo": map[string]any{"name": "fixture", "version": "1"}, "capabilities": map[string]any{"tools": map[string]any{}}}
		case "tools/list":
			result = map[string]any{"resultType": "complete", "ttlMs": 60000, "cacheScope": "private", "tools": []any{}, "nextCursor": "cycle"}
		default:
			t.Errorf("unexpected method %s", request.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer ts.Close()
	client := NewClient(&Config{McpServers: map[string]ServerConfig{"remote": {Type: "http", URL: ts.URL}}}, genieconfig.NewConfigManager())
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, client.ConnectToServers(ctx))
	require.Contains(t, client.ServerErrors()["remote"], "pagination cursor")
	require.Empty(t, client.GetTools())
}
