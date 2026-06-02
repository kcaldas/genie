package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// MaxReadFileSize caps full-file reads. Larger files must be read in slices
// with start_line/end_line so tool output stays bounded and recoverable.
const MaxReadFileSize int64 = 1 * 1024 * 1024 // 1 MiB

// ReadFileTool displays file contents
type ReadFileTool struct {
	publisher events.Publisher
}

// NewReadFileTool creates a new read file tool
func NewReadFileTool(publisher events.Publisher) Tool {
	return &ReadFileTool{
		publisher: publisher,
	}
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
				"start_line": {
					Type:        ai.TypeInteger,
					Description: "Optional 1-indexed inclusive first line to read. Use with end_line to read a slice of a large file. Both must be set together.",
					Minimum:     1,
				},
				"end_line": {
					Type:        ai.TypeInteger,
					Description: "Optional 1-indexed inclusive last line to read. Both start_line and end_line must be set together.",
					Minimum:     1,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status shown in the host UI while this tool runs. Frame it in the user's terms (e.g., 'reading your draft', not 'reading user_notes.md'). Separate channel from your chat reply — don't repeat it there.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"file_path", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "File contents",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the file was read successfully",
				},
				"results": {
					Type:        ai.TypeString,
					Description: "The file contents",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if reading failed",
				},
			},
			Required: []string{"success", "results"},
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
				"results": "",
				"error":   FormatPathOutsideWorkspaceError(ctx, filePath).Error(),
			}, nil
		}
		if err := CheckPathPolicy(ctx, resolvedPath, IntentRead); err != nil {
			return map[string]any{
				"success": false,
				"results": "",
				"error":   err.Error(),
			}, nil
		}
		filePath = resolvedPath

		// Check for required display message and publish event
		if r.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				r.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "readFile",
					Message:  msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		// Check for line numbers option
		showLineNumbers := false
		if lineNumbers, exists := params["line_numbers"]; exists {
			if lineNumbersBool, ok := lineNumbers.(bool); ok {
				showLineNumbers = lineNumbersBool
			}
		}

		// Optional line range. Both ends must be set together; partial
		// specification is an error so we don't silently read more than
		// the model expected.
		startLine, hasStart := numberValue(params, "start_line")
		endLine, hasEnd := numberValue(params, "end_line")
		if hasStart != hasEnd {
			return map[string]any{
				"success": false,
				"results": "",
				"error":   "both start_line and end_line must be provided when reading a range",
			}, nil
		}
		if hasStart {
			if startLine < 1 {
				return map[string]any{
					"success": false,
					"results": "",
					"error":   fmt.Sprintf("start_line must be >= 1 (got %d)", startLine),
				}, nil
			}
			if endLine < startLine {
				return map[string]any{
					"success": false,
					"results": "",
					"error":   fmt.Sprintf("end_line (%d) must be >= start_line (%d)", endLine, startLine),
				}, nil
			}
		}

		info, err := os.Stat(filePath)
		if err != nil {
			message := fmt.Sprintf("failed to stat file: %v", err)
			if os.IsNotExist(err) {
				message = fmt.Sprintf("failed to read file: %v", err)
			}
			return map[string]any{
				"success": false,
				"results": "",
				"error":   message,
			}, nil
		}
		if info.IsDir() {
			return map[string]any{
				"success": false,
				"results": "",
				"error":   "path is a directory, not a file",
			}, nil
		}
		if !hasStart && info.Size() > MaxReadFileSize {
			return map[string]any{
				"success": false,
				"results": "",
				"error": fmt.Sprintf(
					"file is too large to read in full (%d bytes; max %d bytes). To recover, read a smaller slice with start_line and end_line, or search the file first",
					info.Size(),
					MaxReadFileSize,
				),
			}, nil
		}

		// Read file content
		content, err := r.readFileContent(filePath, showLineNumbers, hasStart, int(startLine), int(endLine))
		if err != nil {
			return map[string]any{
				"success": false,
				"results": "",
				"error":   fmt.Sprintf("failed to read file: %v", err),
			}, nil
		}

		return map[string]any{
			"success": true,
			"results": content,
		}, nil
	}
}

// readFileContent reads the file and optionally adds line numbers.
// When rangeRequested is true, only lines [startLine, endLine] (1-indexed,
// inclusive) are returned; line-numbered output preserves the original
// line numbers so the model can refer back to them.
func (r *ReadFileTool) readFileContent(filePath string, showLineNumbers, rangeRequested bool, startLine, endLine int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Increase scanner buffer for files with long lines (logs, minified
	// content). Default 64KB caps at 64K-char single lines which is
	// surprisingly common.
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	first := 1
	last := 0
	if rangeRequested {
		first = startLine
		last = endLine
	}

	var result strings.Builder
	lineNo := 0
	wroteLine := false
	for scanner.Scan() {
		lineNo++
		if rangeRequested && lineNo < first {
			continue
		}
		if rangeRequested && lineNo > last {
			break
		}
		if wroteLine {
			result.WriteString("\n")
		}
		if showLineNumbers {
			result.WriteString(fmt.Sprintf("%6d\t%s", lineNo, scanner.Text()))
		} else {
			result.WriteString(scanner.Text())
		}
		wroteLine = true
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if rangeRequested && !wroteLine {
		return "", fmt.Errorf("start_line %d exceeds file length (%d lines)", startLine, lineNo)
	}
	return result.String(), nil
}

// FormatOutput formats file reading results for user display
func (r *ReadFileTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	content, _ := result["results"].(string)
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
