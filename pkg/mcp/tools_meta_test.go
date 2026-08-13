package mcp

import (
	"testing"

	genieconfig "github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/tools"
)

func TestClientToolsMeta(t *testing.T) {
	config := &Config{McpServers: map[string]ServerConfig{
		"glt": {
			Command:     "glt-mcp",
			DisplayName: "Freight Tools",
			SignalText:  tools.LocalizedText{Default: "Checking freight information"},
			Tools: map[string]ToolDisplayConfig{
				"glt_get_rates": {
					DisplayName: "Get Rates",
					SignalText:  tools.LocalizedText{Default: "Checking rates"},
				},
			},
		},
		"bare": {Command: "bare-mcp"},
	}}

	client := NewClient(config, genieconfig.NewConfigManager())
	client.tools = map[string]*MCPTool{
		"glt_get_rates": {
			mcpTool:    Tool{Name: "glt_get_rates"},
			serverName: "glt",
			client:     client,
		},
		"glt_list_loads": {
			mcpTool:    Tool{Name: "glt_list_loads", Title: "List Loads"},
			serverName: "glt",
			client:     client,
		},
		"bare_tool": {
			mcpTool:    Tool{Name: "bare_tool"},
			serverName: "bare",
			client:     client,
		},
	}

	meta := client.ToolsMeta()
	if len(meta) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(meta))
	}

	// Per-tool config override wins over everything.
	rates := meta["glt_get_rates"]
	if rates.Server != "glt" || rates.ServerDisplayName != "Freight Tools" {
		t.Errorf("server attribution wrong: %+v", rates)
	}
	if rates.DisplayName != "Get Rates" {
		t.Errorf("DisplayName = %q, want config override", rates.DisplayName)
	}
	if got := rates.SignalText.Resolve(""); got != "Checking rates" {
		t.Errorf("SignalText = %q, want per-tool override", got)
	}

	// No per-tool config: server-advertised title + server-level signal text.
	loads := meta["glt_list_loads"]
	if loads.DisplayName != "List Loads" {
		t.Errorf("DisplayName = %q, want server-advertised title", loads.DisplayName)
	}
	if got := loads.SignalText.Resolve(""); got != "Checking freight information" {
		t.Errorf("SignalText = %q, want server-level text", got)
	}

	// No config at all: attribution only, everything else zero.
	bare := meta["bare_tool"]
	if bare.Server != "bare" || bare.ServerDisplayName != "" || bare.DisplayName != "" || !bare.SignalText.IsZero() {
		t.Errorf("bare tool meta must be attribution-only: %+v", bare)
	}
}

func TestRegistryMCPToolsMetaWithoutClient(t *testing.T) {
	registry := tools.NewRegistry()
	if meta := registry.MCPToolsMeta(); len(meta) != 0 {
		t.Fatalf("registry without MCP client must return empty meta, got %v", meta)
	}
}
