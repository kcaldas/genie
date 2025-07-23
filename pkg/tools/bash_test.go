package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
)

func TestBashTool_Declaration(t *testing.T) {
	bashTool := NewBashTool(nil, nil, false)
	
	// Test function declaration
	decl := bashTool.Declaration()
	assert.Equal(t, "bash", decl.Name)
	assert.Contains(t, decl.Description, "bash command")
	assert.NotNil(t, decl.Parameters)
	
	// Test schema structure
	schema := decl.Parameters
	assert.Equal(t, ai.TypeObject, schema.Type)
	assert.Contains(t, schema.Properties, "command")
	assert.Contains(t, schema.Required, "command")
}

func TestBashTool_SimpleCommand(t *testing.T) {
	bashTool := NewBashTool(nil, nil, false)
	handler := bashTool.Handler()
	
	// Test simple echo command
	params := map[string]any{
		"command": "echo 'Hello from bash'",
	}
	
	result, err := handler(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	assert.True(t, result["success"].(bool))
	assert.Contains(t, result["results"].(string), "Hello from bash")
}

func TestBashTool_WithWorkingDirectory(t *testing.T) {
	bashTool := NewBashTool(nil, nil, false)
	handler := bashTool.Handler()
	
	// Test pwd command with working directory
	params := map[string]any{
		"command": "pwd",
		"cwd":     "/tmp",
	}
	
	result, err := handler(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	assert.True(t, result["success"].(bool))
	assert.Contains(t, result["results"].(string), "/tmp")
}

func TestBashTool_CommandTimeout(t *testing.T) {
	bashTool := NewBashTool(nil, nil, false)
	handler := bashTool.Handler()
	
	// Test command with timeout (sleep longer than timeout)
	params := map[string]any{
		"command":    "sleep 5",
		"timeout_ms": float64(100), // 100ms timeout
	}
	
	result, err := handler(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	assert.False(t, result["success"].(bool))
	assert.Contains(t, strings.ToLower(result["error"].(string)), "timed out")
}

func TestBashTool_CommandError(t *testing.T) {
	bashTool := NewBashTool(nil, nil, false)
	handler := bashTool.Handler()
	
	// Test command that fails
	params := map[string]any{
		"command": "exit 1",
	}
	
	result, err := handler(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	assert.False(t, result["success"].(bool))
	assert.NotEmpty(t, result["error"])
}

func TestBashTool_MissingCommand(t *testing.T) {
	bashTool := NewBashTool(nil, nil, false)
	handler := bashTool.Handler()
	
	// Test without command parameter
	params := map[string]any{}
	
	_, err := handler(context.Background(), params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command")
}