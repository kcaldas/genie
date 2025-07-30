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

func TestCleanCommandForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "simple command without HEREDOC",
			command:  "ls -la",
			expected: "ls -la",
		},
		{
			name:     "git status command",
			command:  "git status",
			expected: "git status",
		},
		{
			name:     "complex command with quotes but no HEREDOC",
			command:  `git commit -m "Simple commit message"`,
			expected: `git commit -m "Simple commit message"`,
		},
		{
			name:     "command with pipes and redirects",
			command:  "grep 'error' log.txt | tail -10 > errors.txt",
			expected: "grep 'error' log.txt | tail -10 > errors.txt",
		},
		{
			name:     "command with environment variables",
			command:  "ENV_VAR=value command --flag",
			expected: "ENV_VAR=value command --flag",
		},
		{
			name: "git commit with HEREDOC",
			command: `git commit -m "$(cat <<'EOF'
Add new feature for user authentication

This commit adds login functionality.
EOF
)"`,
			expected: `git commit -m "Add new feature for user authentication

This commit adds login functionality."`,
		},
		{
			name: "git commit with HEREDOC and additional flags",
			command: `git commit -m "$(cat <<'EOF'
Fix bug in user login

Resolves issue with password validation.
EOF
)" --no-verify`,
			expected: `git commit -m "Fix bug in user login

Resolves issue with password validation." --no-verify`,
		},
		{
			name: "gh pr create with HEREDOC",
			command: `gh pr create --title "New Feature" --body "$(cat <<'EOF'
## Summary
- Add user authentication
- Update tests
EOF
)"`,
			expected: `gh pr create --title "New Feature" --body "## Summary
- Add user authentication
- Update tests"`,
		},
		{
			name:     "command with incomplete HEREDOC (missing closing)",
			command:  `git commit -m "$(cat <<'EOF' message here`,
			expected: `git commit -m "$(cat <<'EOF' message here`,
		},
		{
			name:     "command with HEREDOC but no closing parenthesis",
			command:  `git commit -m "$(cat <<'EOF'\nmessage\nEOF"`,
			expected: `git commit -m "$(cat <<'EOF'\nmessage\nEOF"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanCommandForDisplay(tt.command)
			if result != tt.expected {
				t.Errorf("cleanCommandForDisplay() = %q, expected %q", result, tt.expected)
			}
		})
	}
}