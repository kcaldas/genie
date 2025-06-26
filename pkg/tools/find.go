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

// FindTool finds files and directories
type FindTool struct {
	publisher events.Publisher
}

// NewFindTool creates a new find tool
func NewFindTool(publisher events.Publisher) Tool {
	return &FindTool{
		publisher: publisher,
	}
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
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Required message explaining why you are searching for these files. Tell the user what you're looking for or what you plan to do.",
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
		// Check for required display message and publish event
		if f.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				// Get session ID from context if available
				sessionID := ""
				if id, exists := ctx.Value("sessionID").(string); exists {
					sessionID = id
				}
				
				f.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					SessionID: sessionID,
					ToolName:  "findFiles",
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

		// Extract path parameter
		path := "."
		if pathParam, exists := params["path"]; exists {
			if pathStr, ok := pathParam.(string); ok && pathStr != "" {
				path = pathStr
			}
		}

		// Extract working directory from context
		workingDir := "."
		if cwd := ctx.Value("cwd"); cwd != nil {
			if cwdStr, ok := cwd.(string); ok && cwdStr != "" {
				workingDir = cwdStr
			}
		}

		// Resolve relative paths against working directory
		if !strings.HasPrefix(path, "/") {
			path = workingDir + "/" + strings.TrimPrefix(path, "./")
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

		// Check for other errors
		if err != nil {
			return map[string]any{
				"success": false,
				"results": string(output),
				"error":   fmt.Sprintf("find failed: %v", err),
			}, nil
		}

		// Convert absolute paths to relative paths from working directory
		outputStr := string(output)
		if outputStr != "" {
			lines := strings.Split(strings.TrimSpace(outputStr), "\n")
			for i, line := range lines {
				if strings.HasPrefix(line, workingDir) {
					// Convert to relative path
					relPath, _ := strings.CutPrefix(line, workingDir)
					relPath = strings.TrimPrefix(relPath, "/")
					if relPath == "" {
						relPath = "."
					}
					lines[i] = relPath
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

// FormatOutput formats find results for user display
func (f *FindTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	results, _ := result["results"].(string)
	errorMsg, _ := result["error"].(string)
	
	if !success {
		if errorMsg != "" {
			return fmt.Sprintf("**Search failed**: %s", errorMsg)
		}
		return "**Search failed**"
	}
	
	results = strings.TrimSpace(results)
	if results == "" {
		return "**No files found matching criteria**"
	}
	
	// Split results by newline and format as a list
	resultList := strings.Split(results, "\n")
	var formattedResults []string
	for _, item := range resultList {
		item = strings.TrimSpace(item)
		if item != "" {
			// Add appropriate indicator based on file type
			if strings.HasSuffix(item, "/") {
				formattedResults = append(formattedResults, "[DIR]  "+item)
			} else {
				formattedResults = append(formattedResults, "[FILE] "+item)
			}
		}
	}
	
	return fmt.Sprintf("**Search Results**\n%s", strings.Join(formattedResults, "\n"))
}