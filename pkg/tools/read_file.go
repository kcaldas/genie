package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

// ReadFileTool displays file contents
type ReadFileTool struct{}

// NewReadFileTool creates a new read file tool
func NewReadFileTool() Tool {
	return &ReadFileTool{}
}

// Declaration returns the function declaration for the read file tool
func (r *ReadFileTool) Declaration() *ai.FunctionDeclaration {
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

// Handler returns the function handler for the read file tool
func (r *ReadFileTool) Handler() ai.HandlerFunc {
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

		// Check for line numbers option
		showLineNumbers := false
		if lineNumbers, exists := params["line_numbers"]; exists {
			if lineNumbersBool, ok := lineNumbers.(bool); ok {
				showLineNumbers = lineNumbersBool
			}
		}

		// Read file content
		content, err := r.readFileContent(filePath, showLineNumbers)
		if err != nil {
			return map[string]any{
				"success": false,
				"content": "",
				"error":   fmt.Sprintf("failed to read file: %v", err),
			}, nil
		}

		return map[string]any{
			"success": true,
			"content": content,
		}, nil
	}
}

// readFileContent reads the file and optionally adds line numbers
func (r *ReadFileTool) readFileContent(filePath string, showLineNumbers bool) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	
	// Read all lines first
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	// If file is empty, return empty string
	if len(lines) == 0 {
		return "", nil
	}

	var result strings.Builder
	
	// Process lines
	for i, line := range lines {
		if showLineNumbers {
			// Format line numbers similar to cat -n: right-aligned in 6 characters with tab
			result.WriteString(fmt.Sprintf("%6d\t%s", i+1, line))
		} else {
			result.WriteString(line)
		}
		
		// Add newline between lines (but not after the last line)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String(), nil
}

// FormatOutput formats file reading results for user display
func (r *ReadFileTool) FormatOutput(result map[string]interface{}) string {
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