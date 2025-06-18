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
		Name:        "runBashCommand",
		Description: "Execute shell commands for tasks not covered by other specific tools. Use this when you need to run commands that don't have dedicated tools available.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for executing a bash command",
			Properties: map[string]*ai.Schema{
				"command": {
					Type:        ai.TypeString,
					Description: "The shell command to execute. Examples: 'ls -la' to list files, 'git status' to check git status, 'find . -name \"*.go\"' to find Go files, 'ps aux' to check processes",
					MinLength:   1,
					MaxLength:   1000,
				},
				"cwd": {
					Type:        ai.TypeString,
					Description: "Optional working directory to run the command in. Use absolute or relative paths. Example: '/path/to/project' or '.'",
					MaxLength:   500,
				},
				"timeout_ms": {
					Type:        ai.TypeInteger,
					Description: "Optional timeout in milliseconds. Default is 30000ms (30 seconds). Use higher values for long-running commands",
					Minimum:     100,
					Maximum:     300000, // 5 minutes max
				},
			},
			Required: []string{"command"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the bash command execution",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the command executed successfully",
				},
				"output": {
					Type:        ai.TypeString,
					Description: "The command output (stdout and stderr combined)",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if the command failed",
				},
			},
			Required: []string{"success", "output"},
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