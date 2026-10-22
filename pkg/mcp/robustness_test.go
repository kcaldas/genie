package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	genieconfig "github.com/kcaldas/genie/pkg/config"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// Run a real subprocess to exercise stderr draining, cancellation, late replies,
// concurrent request IDs, and shutdown through the production SDK adapter.
func TestMCPSubprocess(t *testing.T) {
	if os.Getenv("GENIE_MCP_TEST_HELPER") != "1" {
		return
	}
	_, _ = os.Stderr.WriteString(strings.Repeat("x", 1<<20))
	server := sdk.NewServer(&sdk.Implementation{Name: "stdio-fixture", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "echo", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(req.Params.Arguments)}}}, nil
	})
	server.AddTool(&sdk.Tool{Name: "wait", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "late"}}}, nil
		}
	})
	if err := server.Run(context.Background(), &sdk.StdioTransport{}); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestStdioCancellationAndConcurrentCalls(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	client := NewClient(&Config{McpServers: map[string]ServerConfig{"local": {Command: executable, Args: []string{"-test.run=^TestMCPSubprocess$"}, Env: map[string]string{"GENIE_MCP_TEST_HELPER": "1"}}}}, genieconfig.NewConfigManager())
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, client.ConnectToServers(ctx))
	require.Empty(t, client.ServerErrors())
	session, err := client.Session("local")
	require.NoError(t, err)
	require.Equal(t, "2026-07-28", session.InitializeResult().ProtocolVersion)
	short, stop := context.WithTimeout(ctx, 25*time.Millisecond)
	_, err = client.CallTool(short, "wait", nil)
	stop()
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			result, err := client.CallTool(ctx, "echo", map[string]any{"value": fmt.Sprintf("message-%02d", i)})
			if err != nil {
				t.Errorf("echo %d: %v", i, err)
				return
			}
			if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, fmt.Sprintf("message-%02d", i)) {
				t.Errorf("mismatched response for %d: %#v", i, result)
			}
		})
	}
	wg.Wait()
	require.NoError(t, client.Close())
	require.Empty(t, client.GetTools())
	_, err = client.CallTool(ctx, "echo", nil)
	require.Error(t, err)
	require.NoError(t, client.Close())
}
