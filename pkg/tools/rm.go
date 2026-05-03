package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// RmTool removes files and directories within the workspace.
type RmTool struct {
	publisher events.Publisher
}

// NewRmTool creates a new rm tool.
func NewRmTool(publisher events.Publisher) Tool {
	return &RmTool{publisher: publisher}
}

// Declaration returns the function declaration for the rm tool.
func (r *RmTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "removeFile",
		Description: "Remove a file or directory inside the workspace. " +
			"Removing a directory requires recursive=\"true\" — without it, " +
			"a directory target is rejected to prevent accidental tree wipes. " +
			"Symlinks are not followed. Refuses to remove the workspace root.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for removing a file or directory",
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "Path to remove (relative to the workspace, or absolute inside the workspace)",
					MinLength:   1,
					MaxLength:   500,
				},
				"recursive": {
					Type:        ai.TypeString,
					Description: "Set to 'true' to remove a directory and its contents. Required for non-empty directories.",
					Enum:        []string{"true", "false"},
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status shown in the host UI while this tool runs. Frame it in the user's terms (e.g., 'cleaning up the temp file'). Separate channel from your chat reply — don't repeat it there.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"path", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the remove",
			Properties: map[string]*ai.Schema{
				"success": {Type: ai.TypeBoolean, Description: "Whether the remove succeeded"},
				"results": {Type: ai.TypeString, Description: "Description of what was removed"},
				"error":   {Type: ai.TypeString, Description: "Error message if the remove failed"},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for the rm tool.
func (r *RmTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if r.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				r.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "removeFile",
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

		recursive := false
		if v, ok := params["recursive"].(string); ok {
			recursive = v == "true"
		}

		resolved, valid := ResolvePathWithWorkingDirectory(ctx, path)
		if !valid {
			return failResult(FormatPathOutsideWorkspaceError(ctx, path).Error()), nil
		}
		if err := CheckPathPolicy(ctx, resolved, IntentMutate); err != nil {
			return failResult(err.Error()), nil
		}

		// Refuse to remove the workspace root itself. Comparing absolute
		// paths catches the case where the model passes "." or the cwd.
		workspace := WorkingDirectoryFromContext(ctx)
		absResolved, errA := filepath.Abs(resolved)
		absWS, errB := filepath.Abs(workspace)
		if errA == nil && errB == nil && absResolved == absWS {
			return failResult("refusing to remove the workspace root"), nil
		}

		info, err := os.Lstat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				return failResult(fmt.Sprintf("path %q does not exist", path)), nil
			}
			return failResult(fmt.Sprintf("stat %q: %v", path, err)), nil
		}

		// Defense in depth: even though the resolver rejects symlink
		// components, a TOCTOU symlink swap on the leaf would still fail
		// here. Lstat catches it.
		if info.Mode()&os.ModeSymlink != 0 {
			return failResult(fmt.Sprintf("path %q is a symlink; refusing to remove", path)), nil
		}

		if info.IsDir() {
			if !recursive {
				return failResult(fmt.Sprintf("path %q is a directory; pass recursive=\"true\" to remove it", path)), nil
			}
			if err := os.RemoveAll(resolved); err != nil {
				return failResult(fmt.Sprintf("remove directory: %v", err)), nil
			}
			return map[string]any{
				"success": true,
				"results": fmt.Sprintf("removed directory %s", path),
			}, nil
		}

		if err := os.Remove(resolved); err != nil {
			return failResult(fmt.Sprintf("remove file: %v", err)), nil
		}
		return map[string]any{
			"success": true,
			"results": fmt.Sprintf("removed file %s", path),
		}, nil
	}
}

// FormatOutput formats the rm result for user display.
func (r *RmTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**Remove failed**: %s", msg)
		}
		return "**Remove failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return msg
	}
	return "Removed."
}
