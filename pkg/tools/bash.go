package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

// BashTool executes bash commands
type BashTool struct{}

// NewBashTool creates a new bash tool
func NewBashTool() Tool {
	return &BashTool{}
}

// Declaration returns the function declaration for the bash tool
func (b *BashTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "bash",
		Description: "Execute a bash command with optional timeout and working directory",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"command": {
					Type:        ai.TypeString,
					Description: "The bash command to execute",
				},
				"cwd": {
					Type:        ai.TypeString,
					Description: "Working directory for the command (optional)",
				},
				"timeout_ms": {
					Type:        ai.TypeInteger,
					Description: "Timeout in milliseconds (optional, default 30000)",
				},
			},
			Required: []string{"command"},
		},
	}
}

// Handler returns the function handler for the bash tool
func (b *BashTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Extract command parameter
		command, ok := params["command"].(string)
		if !ok {
			return nil, fmt.Errorf("command parameter is required and must be a string")
		}

		// Extract optional working directory
		var cwd string
		if cwdParam, exists := params["cwd"]; exists {
			if cwdStr, ok := cwdParam.(string); ok {
				cwd = cwdStr
			}
		}

		// Extract optional timeout
		var timeout time.Duration = 30 * time.Second // Default 30s timeout
		if timeoutParam, exists := params["timeout_ms"]; exists {
			if timeoutMs, ok := timeoutParam.(float64); ok {
				timeout = time.Duration(timeoutMs) * time.Millisecond
			}
		}

		// Create context with timeout
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Create the command
		cmd := exec.CommandContext(execCtx, "bash", "-c", command)
		
		// Set working directory if provided
		if cwd != "" {
			cmd.Dir = cwd
		}

		// Set environment
		cmd.Env = os.Environ()

		// Execute command and capture output
		output, err := cmd.CombinedOutput()
		
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return map[string]any{
				"success": false,
				"output":  string(output),
				"error":   fmt.Sprintf("command timed out after %v", timeout),
			}, nil
		}

		// Check for other errors
		if err != nil {
			return map[string]any{
				"success": false,
				"output":  string(output),
				"error":   fmt.Sprintf("command failed: %v", err),
			}, nil
		}

		return map[string]any{
			"success": true,
			"output":  string(output),
		}, nil
	}
}