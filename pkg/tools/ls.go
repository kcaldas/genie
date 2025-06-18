package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

// LsTool lists files and directories
type LsTool struct{}

// NewLsTool creates a new ls tool
func NewLsTool() Tool {
	return &LsTool{}
}

// Declaration returns the function declaration for the ls tool
func (l *LsTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "listFiles",
		Description: "List files and directories in a path. Use this when you need to see what files and directories exist in a location.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for listing files and directories",
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "Path to list (optional, defaults to current directory). Examples: '.', '/path/to/dir', 'src/'",
					MaxLength:   500,
				},
				"show_hidden": {
					Type:        ai.TypeBoolean,
					Description: "Show hidden files (files starting with .)",
				},
				"long_format": {
					Type:        ai.TypeBoolean,
					Description: "Show detailed information (permissions, size, date)",
				},
			},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "List of files and directories",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the listing was successful",
				},
				"files": {
					Type:        ai.TypeString,
					Description: "The file listing output",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if listing failed",
				},
			},
			Required: []string{"success", "files"},
		},
	}
}

// Handler returns the function handler for the ls tool
func (l *LsTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Extract path parameter
		path := "."
		if pathParam, exists := params["path"]; exists {
			if pathStr, ok := pathParam.(string); ok && pathStr != "" {
				path = pathStr
			}
		}

		// Build ls command
		args := []string{}
		
		// Check for long format
		if longFormat, exists := params["long_format"]; exists {
			if longFormatBool, ok := longFormat.(bool); ok && longFormatBool {
				args = append(args, "-l")
			}
		}

		// Check for hidden files
		if showHidden, exists := params["show_hidden"]; exists {
			if showHiddenBool, ok := showHidden.(bool); ok && showHiddenBool {
				args = append(args, "-a")
			}
		}

		// Add path
		args = append(args, path)

		// Create context with timeout
		execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Execute ls command
		cmd := exec.CommandContext(execCtx, "ls", args...)
		cmd.Env = os.Environ()

		output, err := cmd.CombinedOutput()
		
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return map[string]any{
				"success": false,
				"files":   string(output),
				"error":   "command timed out",
			}, nil
		}

		// Check for other errors
		if err != nil {
			return map[string]any{
				"success": false,
				"files":   string(output),
				"error":   fmt.Sprintf("ls failed: %v", err),
			}, nil
		}

		return map[string]any{
			"success": true,
			"files":   string(output),
		}, nil
	}
}