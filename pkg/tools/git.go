package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

// GitStatusTool shows git repository status
type GitStatusTool struct{}

// NewGitStatusTool creates a new git status tool
func NewGitStatusTool() Tool {
	return &GitStatusTool{}
}

// Declaration returns the function declaration for the git status tool
func (g *GitStatusTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "gitStatus",
		Description: "Show the status of the git repository including staged, unstaged, and untracked files. Use this when you need to understand the current state of the git repository.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for git status",
			Properties: map[string]*ai.Schema{
				"short": {
					Type:        ai.TypeBoolean,
					Description: "Show short format output (more concise)",
				},
				"branch": {
					Type:        ai.TypeBoolean,
					Description: "Show branch information",
				},
			},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Git repository status",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether git status was successful",
				},
				"status": {
					Type:        ai.TypeString,
					Description: "The git status output",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if git status failed",
				},
			},
			Required: []string{"success", "status"},
		},
	}
}

// Handler returns the function handler for the git status tool
func (g *GitStatusTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Build git status command
		args := []string{"status"}

		// Check for short format
		if short, exists := params["short"]; exists {
			if shortBool, ok := short.(bool); ok && shortBool {
				args = append(args, "--short")
			}
		}

		// Check for branch info
		if branch, exists := params["branch"]; exists {
			if branchBool, ok := branch.(bool); ok && branchBool {
				args = append(args, "--branch")
			}
		}

		// Create context with timeout
		execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Execute git status command
		cmd := exec.CommandContext(execCtx, "git", args...)
		cmd.Env = os.Environ()

		output, err := cmd.CombinedOutput()
		
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return map[string]any{
				"success": false,
				"status":  string(output),
				"error":   "git status timed out",
			}, nil
		}

		// Check for other errors
		if err != nil {
			return map[string]any{
				"success": false,
				"status":  string(output),
				"error":   fmt.Sprintf("git status failed: %v", err),
			}, nil
		}

		return map[string]any{
			"success": true,
			"status":  string(output),
		}, nil
	}
}