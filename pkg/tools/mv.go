package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// MvTool moves (renames) files and directories within the workspace.
type MvTool struct {
	publisher events.Publisher
}

// NewMvTool creates a new mv tool.
func NewMvTool(publisher events.Publisher) Tool {
	return &MvTool{publisher: publisher}
}

// Declaration returns the function declaration for the mv tool.
func (m *MvTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "moveFile",
		Description: "Move or rename a file or directory within the workspace. " +
			"Both source and destination must be inside the workspace (or an allowed directory). " +
			"Refuses to overwrite an existing destination unless overwrite=\"true\". " +
			"Symlinks are not followed and will be rejected.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for moving a file or directory",
			Properties: map[string]*ai.Schema{
				"source": {
					Type:        ai.TypeString,
					Description: "Source path (relative to the workspace, or absolute inside the workspace)",
					MinLength:   1,
					MaxLength:   500,
				},
				"destination": {
					Type:        ai.TypeString,
					Description: "Destination path (relative to the workspace, or absolute inside the workspace)",
					MinLength:   1,
					MaxLength:   500,
				},
				"overwrite": {
					Type:        ai.TypeString,
					Description: "Set to 'true' to overwrite an existing destination. Defaults to 'false'.",
					Enum:        []string{"true", "false"},
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status shown in the host UI while this tool runs. Frame it in the user's terms (e.g., 'renaming the report'). Separate channel from your chat reply — don't repeat it there.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"source", "destination", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the move",
			Properties: map[string]*ai.Schema{
				"success": {Type: ai.TypeBoolean, Description: "Whether the move succeeded"},
				"results": {Type: ai.TypeString, Description: "Description of what was moved"},
				"error":   {Type: ai.TypeString, Description: "Error message if the move failed"},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for the mv tool.
func (m *MvTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if m.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				m.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "moveFile",
					Message:  msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		source, ok := params["source"].(string)
		if !ok || source == "" {
			return failResult("source parameter is required and must be a non-empty string"), nil
		}
		destination, ok := params["destination"].(string)
		if !ok || destination == "" {
			return failResult("destination parameter is required and must be a non-empty string"), nil
		}

		overwrite := false
		if v, ok := params["overwrite"].(string); ok {
			overwrite = v == "true"
		}

		resolvedSrc, valid := ResolvePathWithWorkingDirectory(ctx, source)
		if !valid {
			return failResult(FormatPathOutsideWorkspaceError(ctx, source).Error()), nil
		}
		resolvedDst, valid := ResolvePathWithWorkingDirectory(ctx, destination)
		if !valid {
			return failResult(FormatPathOutsideWorkspaceError(ctx, destination).Error()), nil
		}

		if err := movePath(resolvedSrc, resolvedDst, overwrite); err != nil {
			return failResult(err.Error()), nil
		}

		return map[string]any{
			"success": true,
			"results": fmt.Sprintf("moved %s to %s", source, destination),
		}, nil
	}
}

// FormatOutput formats the mv result for user display.
func (m *MvTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**Move failed**: %s", msg)
		}
		return "**Move failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return msg
	}
	return "Moved."
}

// movePath renames src to dst within the workspace. Refuses symlinks and
// refuses to overwrite unless overwrite is true. Falls back to copy+delete
// when os.Rename fails (e.g. cross-filesystem moves) so the operation
// works regardless of where the workspace is mounted.
func movePath(src, dst string, overwrite bool) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("source %q: %w", src, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source %q is a symlink; refusing to move", src)
	}

	if dstInfo, err := os.Lstat(dst); err == nil {
		if !overwrite {
			return fmt.Errorf("destination %q already exists; pass overwrite=\"true\" to replace it", dst)
		}
		if dstInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination %q is a symlink; refusing to overwrite", dst)
		}
		if err := os.RemoveAll(dst); err != nil {
			return fmt.Errorf("remove existing destination: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat destination: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("ensure destination parent: %w", err)
	}

	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Cross-device or other rename failure: fall back to copy + remove.
	if err := copyPath(src, dst, overwrite); err != nil {
		return fmt.Errorf("rename failed and fallback copy failed: %w", err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("copied to destination but failed to remove source: %w", err)
	}
	return nil
}
