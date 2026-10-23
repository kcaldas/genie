package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	genieconfig "github.com/kcaldas/genie/pkg/config"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestToolHandlerSendsEmptyObjectForNilArguments(t *testing.T) {
	server := sdk.NewServer(&sdk.Implementation{Name: "strict", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "no_args", InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		if string(req.Params.Arguments) != "{}" {
			return nil, fmt.Errorf("expected empty argument object, got %s", req.Params.Arguments)
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "ok"}}}, nil
	})
	ts := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true}))
	defer ts.Close()
	client := NewClient(&Config{McpServers: map[string]ServerConfig{"strict": {Type: "http", URL: ts.URL}}}, genieconfig.NewConfigManager())
	defer client.Close()
	require.NoError(t, client.ConnectToServers(context.Background()))
	require.Empty(t, client.ServerErrors())
	require.Len(t, client.GetTools(), 1)
	result, err := client.GetTools()[0].Handler()(context.Background(), nil)
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestInitReportsInvalidConfigWithoutFailing(t *testing.T) {
	for _, tt := range []struct{ name, config, key string }{
		{"malformed JSON", `{"mcpServers":`, "<config>"},
		{"unknown transport", `{"mcpServers":{"broken":{"type":"typo"}}}`, "broken"},
		{"bad prefix", `{"mcpServers":{"broken":{"command":"unused","toolPrefix":"bad prefix"}}}`, "broken"},
		{"protocol header", `{"mcpServers":{"broken":{"type":"http","url":"https://example.com/mcp","headers":{"Mcp-Method":"tools/call"}}}}`, "broken"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(tt.config), 0600))
			client := NewClient(nil, genieconfig.NewConfigManager())
			defer client.Close()
			require.NoError(t, client.Init(dir))
			require.NotEmpty(t, client.ServerErrors()[tt.key])
			require.Empty(t, client.GetTools())
		})
	}
}

func TestInitMCPConfigDirReportsLookupErrors(t *testing.T) {
	for _, scenario := range []string{"missing", "permission denied", "symlink loop"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".mcp.json")
			switch scenario {
			case "permission denied":
				require.NoError(t, os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0600))
				require.NoError(t, os.Chmod(dir, 0000))
				t.Cleanup(func() { require.NoError(t, os.Chmod(dir, 0700)) })
				if _, err := os.Stat(path); !errors.Is(err, os.ErrPermission) {
					t.Skip("environment does not enforce directory permissions")
				}
			case "symlink loop":
				if err := os.Symlink(".mcp.json", path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				_, err := os.Stat(path)
				require.Error(t, err)
				require.False(t, errors.Is(err, os.ErrNotExist))
			}
			client := NewClient(nil, genieconfig.NewConfigManager())
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			require.NoError(t, client.InitMCPConfigDir(dir), "lookup errors must not fail startup")
			require.Empty(t, client.GetTools())
			if scenario == "missing" {
				require.Empty(t, client.ServerErrors())
			} else {
				require.Contains(t, client.ServerErrors()["<config>"], path)
				require.Contains(t, client.ServerErrors()["<config>"], "locate MCP configuration")
			}
		})
	}
}

func TestSameOriginDefaultPorts(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		same bool
	}{
		{"https://example.com/mcp", "https://EXAMPLE.com:443/other", true},
		{"http://example.com", "http://example.com:80", true},
		{"https://[::1]", "https://[::1]:443", true},
		{"https://example.com", "https://example.com:444", false},
		{"https://example.com", "http://example.com:443", false},
		{"https://example.com", "https://other.example.com", false},
	} {
		t.Run(tt.a+" -> "+tt.b, func(t *testing.T) {
			a, err := url.Parse(tt.a)
			require.NoError(t, err)
			b, err := url.Parse(tt.b)
			require.NoError(t, err)
			require.Equal(t, tt.same, sameOrigin(a, b))
		})
	}
}

// A raw catalog avoids the SDK server's own notification debounce and cache,
// making refresh request counts and a failure during discovery deterministic.
func newCatalogTestClient(t *testing.T, list func(http.ResponseWriter, *http.Request, int64) bool) (*Client, *atomic.Int64) {
	t.Helper()
	calls := &atomic.Int64{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     any
			Method string
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		var result any
		switch req.Method {
		case "server/discover":
			result = map[string]any{"resultType": "complete", "supportedVersions": []string{"2026-07-28"}, "serverInfo": map[string]any{"name": "catalog", "version": "1"}, "capabilities": map[string]any{"tools": map[string]any{}}}
		case "tools/list":
			n := calls.Add(1)
			if !list(w, r, n) {
				return
			}
			result = map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private", "tools": []any{map[string]any{"name": "echo", "description": fmt.Sprintf("catalog %d", n), "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "text", "text": "ok"}}}
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(ts.Close)
	client := NewClient(&Config{McpServers: map[string]ServerConfig{"catalog": {Type: "http", URL: ts.URL}}}, genieconfig.NewConfigManager())
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, client.ConnectToServers(context.Background()))
	require.Empty(t, client.ServerErrors())
	return client, calls
}

func waitForCatalogRefresh(t *testing.T, client *Client, conn *ServerConnection) {
	t.Helper()
	require.Eventually(t, func() bool {
		client.mu.RLock()
		defer client.mu.RUnlock()
		return !conn.refreshRunning
	}, 3*time.Second, time.Millisecond)
}

func TestCatalogRefreshFailureKeepsConnectionAndRecovers(t *testing.T) {
	var fail atomic.Bool
	client, _ := newCatalogTestClient(t, func(w http.ResponseWriter, _ *http.Request, _ int64) bool {
		if fail.Load() {
			http.Error(w, "catalog unavailable", http.StatusServiceUnavailable)
			return false
		}
		return true
	})
	conn := client.servers["catalog"]
	old := client.GetTools()[0]
	fail.Store(true)
	client.scheduleToolsRefresh(conn)
	waitForCatalogRefresh(t, client, conn)
	require.Empty(t, client.ServerErrors())
	require.NotEmpty(t, client.RefreshErrors()["catalog"])
	errors := client.RefreshErrors()
	delete(errors, "catalog")
	require.NotEmpty(t, client.RefreshErrors()["catalog"], "diagnostics must be a snapshot")
	require.Equal(t, old.Declaration().Description, client.GetTools()[0].Declaration().Description)
	_, err := old.Handler()(context.Background(), nil)
	require.NoError(t, err, "the connected server remains callable after a failed refresh")
	fail.Store(false)
	client.scheduleToolsRefresh(conn)
	waitForCatalogRefresh(t, client, conn)
	require.Empty(t, client.RefreshErrors())
	require.Empty(t, client.ServerErrors())
	require.Equal(t, "catalog 3", client.GetTools()[0].Declaration().Description)
}

func TestToolChangeBurstCoalescesAndKeepsLatestCatalog(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	client, calls := newCatalogTestClient(t, func(_ http.ResponseWriter, r *http.Request, n int64) bool {
		if n == 2 {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
				return false
			}
		}
		return true
	})
	conn := client.servers["catalog"]
	client.scheduleToolsRefresh(conn)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("refresh did not start")
	}
	// The first response is in flight: all changes must schedule one follow-up,
	// without losing changes that arrive during that in-flight request.
	for range 1000 {
		client.scheduleToolsRefresh(conn)
	}
	unblock()
	waitForCatalogRefresh(t, client, conn)
	require.EqualValues(t, 3, calls.Load(), "initial discovery, active refresh, and one follow-up")
	require.Equal(t, "catalog 3", client.GetTools()[0].Declaration().Description)
	require.Empty(t, client.RefreshErrors())
	require.NoError(t, client.Close())
	client.scheduleToolsRefresh(conn)
	client.mu.RLock()
	require.False(t, conn.refreshRunning, "closed clients must not start refresh workers")
	client.mu.RUnlock()
}

func TestToolConflictsWarnWithServerNames(t *testing.T) {
	// No network is needed to verify the diagnostic emitted when catalogs merge.
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)
	client := NewClient(nil, genieconfig.NewConfigManager())
	for _, name := range []string{"a", "b", "c"} {
		client.servers[name] = &ServerConnection{name: name, tools: []Tool{{Name: "echo"}}}
	}
	client.rebuildToolsLocked()
	require.Empty(t, client.GetTools())
	require.Empty(t, client.ServerErrors(), "name collisions do not mean a connection failed")
	var warning struct {
		Level   string
		Msg     string
		Tool    string
		Servers []string
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &warning))
	require.Equal(t, "WARN", warning.Level)
	require.Equal(t, "echo", warning.Tool)
	require.Equal(t, []string{"a", "b", "c"}, warning.Servers)
	require.Contains(t, warning.Msg, "toolPrefix")
}
