package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

// CatTool displays file contents
type CatTool struct{}

// NewCatTool creates a new cat tool
func NewCatTool() Tool {
	return &CatTool{}
}

// Declaration returns the function declaration for the cat tool
func (c *CatTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "readFile",
		Description: "Read and display the contents of a file. Use this when you need to see what's inside a file or examine file contents.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for reading a file",
			Properties: map[string]*ai.Schema{
				"file_path": {
					Type:        ai.TypeString,
					Description: "Path to the file to read. Examples: 'README.md', 'src/main.go', 'config.json'",
					MinLength:   1,
					MaxLength:   500,
				},
				"line_numbers": {
					Type:        ai.TypeBoolean,
					Description: "Show line numbers in the output",
				},
			},
			Required: []string{"file_path"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "File contents",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the file was read successfully",
				},
				"content": {
					Type:        ai.TypeString,
					Description: "The file contents",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if reading failed",
				},
			},
			Required: []string{"success", "content"},
		},
	}
}

// Handler returns the function handler for the cat tool
func (c *CatTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Extract file path parameter
		filePath, ok := params["file_path"].(string)
		if !ok || filePath == "" {
			return nil, fmt.Errorf("file_path parameter is required and must be a non-empty string")
		}

		// Resolve path with working directory
		resolvedPath, isValid := ResolvePathWithWorkingDirectory(ctx, filePath)
		if !isValid {
			return map[string]any{
				"success": false,
				"content": "",
				"error":   "file path is outside working directory",
			}, nil
		}
		filePath = resolvedPath
		
		// Extract working directory for command execution
		workingDir := "."
		if cwd := ctx.Value("cwd"); cwd != nil {
			if cwdStr, ok := cwd.(string); ok && cwdStr != "" {
				workingDir = cwdStr
			}
		}

		// Build cat command
		args := []string{}

		// Check for line numbers
		if lineNumbers, exists := params["line_numbers"]; exists {
			if lineNumbersBool, ok := lineNumbers.(bool); ok && lineNumbersBool {
				args = append(args, "-n")
			}
		}

		// Add file path
		args = append(args, filePath)

		// Create context with timeout
		execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Execute cat command
		cmd := exec.CommandContext(execCtx, "cat", args...)
		cmd.Env = os.Environ()
		cmd.Dir = workingDir

		output, err := cmd.CombinedOutput()
		
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return map[string]any{
				"success": false,
				"content": string(output),
				"error":   "reading file timed out",
			}, nil
		}

		// Check for other errors
		if err != nil {
			return map[string]any{
				"success": false,
				"content": string(output),
				"error":   fmt.Sprintf("failed to read file: %v", err),
			}, nil
		}

		return map[string]any{
			"success": true,
			"content": string(output),
		}, nil
	}
}

// FormatOutput formats file reading results for user display
func (c *CatTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	content, _ := result["content"].(string)
	errorMsg, _ := result["error"].(string)
	
	if !success {
		if errorMsg != "" {
			return fmt.Sprintf("**Failed to read file**: %s", errorMsg)
		}
		return "**Failed to read file**"
	}
	
	content = strings.TrimSpace(content)
	if content == "" {
		return "**File is empty**"
	}
	
	// Truncate very long content for display
	if len(content) > 1000 {
		content = content[:1000] + "\n... (truncated for display)"
	}
	
	return fmt.Sprintf("**File Content**\n```\n%s\n```", content)
}