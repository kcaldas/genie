package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// GrepTool searches for patterns in files
type GrepTool struct {
	publisher events.Publisher
}

// NewGrepTool creates a new grep tool
func NewGrepTool(publisher events.Publisher) Tool {
	return &GrepTool{
		publisher: publisher,
	}
}

// Declaration returns the function declaration for the grep tool
func (g *GrepTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "searchInFiles",
		Description: "Search for text patterns within files. Use this when you need to find specific content, function definitions, or text patterns across files.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for searching within files",
			Properties: map[string]*ai.Schema{
				"pattern": {
					Type:        ai.TypeString,
					Description: "Text pattern to search for. Examples: 'func main', 'TODO', 'error', 'import'",
					MinLength:   1,
					MaxLength:   200,
				},
				"path": {
					Type:        ai.TypeString,
					Description: "Path to search in (optional, defaults to current directory)",
					MaxLength:   500,
				},
				"file_pattern": {
					Type:        ai.TypeString,
					Description: "File pattern to limit search. Examples: '*.go', '*.js', '*.md'",
					MaxLength:   50,
				},
				"case_sensitive": {
					Type:        ai.TypeBoolean,
					Description: "Whether search should be case sensitive",
				},
				"line_numbers": {
					Type:        ai.TypeBoolean,
					Description: "Show line numbers in results",
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Required message explaining why you are searching for this pattern. Tell the user what you're looking for or investigating.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"pattern", "_display_message"},
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
					Description: "Found matches with file names and line numbers",
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

// Handler returns the function handler for the grep tool
func (g *GrepTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Check for required display message and publish event
		if g.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				// Get session ID from context if available
				sessionID := ""
				if id, exists := ctx.Value("sessionID").(string); exists {
					sessionID = id
				}
				
				g.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					SessionID: sessionID,
					ToolName:  "searchInFiles",
					Message:   msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		// Extract pattern parameter
		pattern, ok := params["pattern"].(string)
		if !ok || pattern == "" {
			return nil, fmt.Errorf("pattern parameter is required and must be a non-empty string")
		}

		// Build grep command
		args := []string{"-r"} // Recursive by default

		// Check for line numbers
		if lineNumbers, exists := params["line_numbers"]; exists {
			if lineNumbersBool, ok := lineNumbers.(bool); ok && lineNumbersBool {
				args = append(args, "-n")
			}
		} else {
			args = append(args, "-n") // Default to showing line numbers
		}

		// Check for case sensitivity
		if caseSensitive, exists := params["case_sensitive"]; exists {
			if caseSensitiveBool, ok := caseSensitive.(bool); ok && !caseSensitiveBool {
				args = append(args, "-i")
			}
		}

		// Add pattern
		args = append(args, pattern)

		// Extract working directory from context
		workingDir := "."
		if cwd := ctx.Value("cwd"); cwd != nil {
			if cwdStr, ok := cwd.(string); ok && cwdStr != "" {
				workingDir = cwdStr
			}
		}

		// Add path
		path := "."
		if pathParam, exists := params["path"]; exists {
			if pathStr, ok := pathParam.(string); ok && pathStr != "" {
				path = pathStr
			}
		}
		
		// Resolve relative paths against working directory
		if !strings.HasPrefix(path, "/") {
			path = workingDir + "/" + strings.TrimPrefix(path, "./")
		}
		
		args = append(args, path)

		// Add file pattern if specified
		if filePattern, exists := params["file_pattern"]; exists {
			if filePatternStr, ok := filePattern.(string); ok && filePatternStr != "" {
				args = append(args, "--include="+filePatternStr)
			}
		}

		// Create context with timeout
		execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Execute grep command
		cmd := exec.CommandContext(execCtx, "grep", args...)
		cmd.Env = os.Environ()
		cmd.Dir = workingDir

		output, err := cmd.CombinedOutput()
		
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return map[string]any{
				"success": false,
				"results": string(output),
				"error":   "search timed out",
			}, nil
		}

		// Grep returns exit code 1 when no matches found, which is not an error
		if err != nil && len(output) == 0 {
			return map[string]any{
				"success": true,
				"results": "No matches found",
			}, nil
		}

		// Convert absolute paths to relative paths from working directory
		outputStr := string(output)
		if outputStr != "" {
			lines := strings.Split(strings.TrimSpace(outputStr), "\n")
			for i, line := range lines {
				// Grep output format is typically: path:line_number:content
				// We need to convert the path part to relative
				if colonIndex := strings.Index(line, ":"); colonIndex > 0 {
					pathPart := line[:colonIndex]
					restPart := line[colonIndex:]
					
					if strings.HasPrefix(pathPart, workingDir) {
						// Convert to relative path
						relPath, _ := strings.CutPrefix(pathPart, workingDir)
						relPath = strings.TrimPrefix(relPath, "/")
						if relPath == "" {
							relPath = "."
						}
						lines[i] = relPath + restPart
					}
				}
			}
			outputStr = strings.Join(lines, "\n")
		}

		return map[string]any{
			"success": true,
			"results": outputStr,
		}, nil
	}
}

// FormatOutput formats grep search results for user display
func (g *GrepTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	matches, _ := result["results"].(string)
	errorMsg, _ := result["error"].(string)
	
	if !success {
		if errorMsg != "" {
			return fmt.Sprintf("**Search failed**: %s", errorMsg)
		}
		return "**Search failed**"
	}
	
	matches = strings.TrimSpace(matches)
	if matches == "" {
		return "**No matches found**"
	}
	
	// Format grep output with syntax highlighting indication
	return fmt.Sprintf("**Search Matches**\n```\n%s\n```", matches)
}