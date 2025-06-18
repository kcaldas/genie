package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

// FindTool finds files and directories
type FindTool struct{}

// NewFindTool creates a new find tool
func NewFindTool() Tool {
	return &FindTool{}
}

// Declaration returns the function declaration for the find tool
func (f *FindTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "findFiles",
		Description: "Search for files and directories by name pattern. Use this when you need to locate specific files or find files matching a pattern.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for finding files",
			Properties: map[string]*ai.Schema{
				"pattern": {
					Type:        ai.TypeString,
					Description: "File name pattern to search for. Examples: '*.go', '*.js', 'main.go', '*test*', 'config.*'",
					MinLength:   1,
					MaxLength:   100,
				},
				"path": {
					Type:        ai.TypeString,
					Description: "Starting directory to search from (optional, defaults to current directory)",
					MaxLength:   500,
				},
				"type": {
					Type:        ai.TypeString,
					Description: "Type of items to find: 'file', 'directory', or 'any' (default)",
					Enum:        []string{"file", "directory", "any"},
				},
			},
			Required: []string{"pattern"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Search results",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the search was successful",
				},
				"results": {
					Type:        ai.TypeString,
					Description: "Found files and directories",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if search failed",
				},
			},
			Required: []string{"success", "results"},
		},
	}
}

// Handler returns the function handler for the find tool
func (f *FindTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Extract pattern parameter
		pattern, ok := params["pattern"].(string)
		if !ok || pattern == "" {
			return nil, fmt.Errorf("pattern parameter is required and must be a non-empty string")
		}

		// Extract path parameter
		path := "."
		if pathParam, exists := params["path"]; exists {
			if pathStr, ok := pathParam.(string); ok && pathStr != "" {
				path = pathStr
			}
		}

		// Build find command
		args := []string{path}

		// Add type filter if specified
		if typeParam, exists := params["type"]; exists {
			if typeStr, ok := typeParam.(string); ok {
				switch typeStr {
				case "file":
					args = append(args, "-type", "f")
				case "directory":
					args = append(args, "-type", "d")
				}
			}
		}

		// Add name pattern
		args = append(args, "-name", pattern)

		// Create context with timeout
		execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Execute find command
		cmd := exec.CommandContext(execCtx, "find", args...)
		cmd.Env = os.Environ()

		output, err := cmd.CombinedOutput()
		
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return map[string]any{
				"success": false,
				"results": string(output),
				"error":   "search timed out",
			}, nil
		}

		// Check for other errors
		if err != nil {
			return map[string]any{
				"success": false,
				"results": string(output),
				"error":   fmt.Sprintf("find failed: %v", err),
			}, nil
		}

		return map[string]any{
			"success": true,
			"results": string(output),
		}, nil
	}
}