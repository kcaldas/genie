package genie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/tools"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestMCPConfigDirectoryIsIndependentOfFileWorkspace(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	workspace := filepath.Join(root, "users", "alice")
	require.NoError(t, os.MkdirAll(workspace, 0700))
	t.Chdir(root)
	server := sdk.NewServer(&sdk.Implementation{Name: "host", Version: "1"}, nil)
	sdk.AddTool(server, &sdk.Tool{Name: "remote_echo", Description: "Echo"}, func(context.Context, *sdk.CallToolRequest, struct{}) (*sdk.CallToolResult, map[string]any, error) {
		return nil, map[string]any{"ok": true}, nil
	})
	endpoint := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	defer endpoint.Close()
	data, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"host": map[string]string{"type": "http", "url": endpoint.URL}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".mcp.json"), data, 0600))
	// The workspace's config is never read when the agent directory is explicit.
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0600))
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "default workspace", true: "host config"}[explicit], func(t *testing.T) {
			g, err := NewGenie(WithToolInterceptor(tools.InterceptorFunc(func(_ context.Context, call tools.ToolCall) (tools.ToolCall, error) {
				return call, nil
			})))
			require.NoError(t, err)
			defer g.Shutdown()
			opts := []StartOption{WithPersonaYAML([]byte("name: test\ninstruction: Test\ntext: '{{.message}}'\n"))}
			if explicit {
				opts = append(opts, WithMCPConfigDir(".")) // Relative to process cwd, not workspace.
			}
			session, err := g.Start(&workspace, nil, opts...)
			require.NoError(t, err)
			registry, err := g.GetToolsRegistry()
			require.NoError(t, err)
			_, found := registry.Get("remote_echo")
			require.Equal(t, explicit, found)
			require.Equal(t, workspace, session.GetWorkingDirectory())
			require.NotContains(t, session.GetAllowedDirectories(), root)
			_, allowed := tools.ResolvePathWithWorkingDirectory(applySessionContext(context.Background(), session), filepath.Join(root, ".mcp.json"))
			require.False(t, allowed, "MCP config selection must not widen file access")
		})
	}
}

func TestMCPConfigDirectoryMustExist(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	g, err := NewGenie()
	require.NoError(t, err)
	defer g.Shutdown()
	_, err = g.Start(&root, nil, WithMCPConfigDir(filepath.Join(root, "missing")))
	require.ErrorContains(t, err, "MCP config directory")
}

func TestMCPConfigDirectoryMustBeDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	path := filepath.Join(root, "file")
	require.NoError(t, os.WriteFile(path, nil, 0600))
	g, err := NewGenie()
	require.NoError(t, err)
	defer g.Shutdown()
	_, err = g.Start(&root, nil, WithMCPConfigDir(path))
	require.ErrorContains(t, err, path)
	require.ErrorContains(t, err, "is not a directory")
}

// Embedding only Registry hides the optional explicit-directory capability.
type registryWithoutMCPConfigDir struct{ tools.Registry }

func TestExplicitMCPConfigDirectoryRequiresRegistrySupport(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	g, err := NewGenie(WithToolRegistry(registryWithoutMCPConfigDir{tools.NewRegistry()}))
	require.NoError(t, err)
	defer g.Shutdown()
	_, err = g.Start(&root, nil, WithMCPConfigDir(root))
	require.ErrorContains(t, err, "tool registry does not support explicit MCP configuration directories")
}

func TestExplicitMCPConfigDirectoryNeverFallsBack(t *testing.T) {
	for _, homeConfig := range []string{filepath.Join(".config", "claude", "mcp.json"), ".mcp.json"} {
		t.Run(homeConfig, func(t *testing.T) {
			root := t.TempDir() // Intentionally has no .mcp.json.
			workspace := t.TempDir()
			home := t.TempDir()
			t.Chdir(root)
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			path := filepath.Join(home, homeConfig)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
			// Reading either untrusted location would report a config error.
			require.NoError(t, os.WriteFile(path, []byte("invalid home config"), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(workspace, ".mcp.json"), []byte("invalid workspace config"), 0600))
			for _, explicit := range []bool{false, true} {
				g, err := NewGenie()
				require.NoError(t, err)
				defer g.Shutdown()
				opts := []StartOption{WithPersonaYAML([]byte("name: test\ninstruction: Test\ntext: '{{.message}}'\n"))}
				workingDir := root
				if explicit {
					workingDir = workspace
					opts = append(opts, WithMCPConfigDir(root))
				}
				_, err = g.Start(&workingDir, nil, opts...)
				require.NoError(t, err)
				registry, err := g.GetToolsRegistry()
				require.NoError(t, err)
				if explicit {
					require.Empty(t, registry.MCPServerErrors())
				} else {
					require.Contains(t, registry.MCPServerErrors(), "<config>", "default discovery still reads the home config")
				}
			}
		})
	}
}
