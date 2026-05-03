package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// MkdirTool creates directories within the workspace.
type MkdirTool struct {
	publisher events.Publisher
}

// NewMkdirTool creates a new mkdir tool.
func NewMkdirTool(publisher events.Publisher) Tool {
	return &MkdirTool{publisher: publisher}
}

// Declaration returns the function declaration for the mkdir tool.
func (m *MkdirTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "makeDirectory",
		Description: "Create a directory inside the workspace. Idempotent: " +
			"succeeds with a clear note if the directory already exists. Parent " +
			"directories are created as needed. Refuses to operate on a path with " +
			"any symlink component.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for creating a directory",
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "Directory path to create (relative to the workspace, or absolute inside the workspace)",
					MinLength:   1,
					MaxLength:   500,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status shown in the host UI while this tool runs. Frame it in the user's terms (e.g., 'creating a folder for the project notes'). Separate channel from your chat reply — don't repeat it there.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"path", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the directory creation",
			Properties: map[string]*ai.Schema{
				"success": {Type: ai.TypeBoolean, Description: "Whether the operation succeeded"},
				"results": {Type: ai.TypeString, Description: "Description of what was done"},
				"error":   {Type: ai.TypeString, Description: "Error message if the operation failed"},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for the mkdir tool.
func (m *MkdirTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if m.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				m.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "makeDirectory",
					Message:  msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		path, ok := params["path"].(string)
		if !ok || path == "" {
			return failResult("path parameter is required and must be a non-empty string"), nil
		}

		resolved, valid := ResolvePathWithWorkingDirectory(ctx, path)
		if !valid {
			return failResult(FormatPathOutsideWorkspaceError(ctx, path).Error()), nil
		}
		if err := CheckPathPolicy(ctx, resolved, IntentMutate); err != nil {
			return failResult(err.Error()), nil
		}

		// Already exists?
		if info, err := os.Lstat(resolved); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return failResult(fmt.Sprintf("path %q is a symlink; refusing to operate", path)), nil
			}
			if info.IsDir() {
				return map[string]any{
					"success": true,
					"results": fmt.Sprintf("directory %s already exists", path),
				}, nil
			}
			return failResult(fmt.Sprintf("path %q exists and is a file, not a directory", path)), nil
		} else if !os.IsNotExist(err) {
			return failResult(fmt.Sprintf("stat %q: %v", path, err)), nil
		}

		if err := os.MkdirAll(resolved, 0o755); err != nil {
			return failResult(fmt.Sprintf("create directory: %v", err)), nil
		}
		return map[string]any{
			"success": true,
			"results": fmt.Sprintf("created directory %s", path),
		}, nil
	}
}

// FormatOutput formats the mkdir result for user display.
func (m *MkdirTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**Create directory failed**: %s", msg)
		}
		return "**Create directory failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return msg
	}
	return "Directory created."
}
