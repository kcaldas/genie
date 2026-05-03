package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// CpTool copies files and directories within the workspace.
type CpTool struct {
	publisher events.Publisher
}

// NewCpTool creates a new cp tool.
func NewCpTool(publisher events.Publisher) Tool {
	return &CpTool{publisher: publisher}
}

// Declaration returns the function declaration for the cp tool.
func (c *CpTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "copyFile",
		Description: "Copy a file or directory within the workspace. " +
			"Both source and destination must be inside the workspace (or an allowed directory). " +
			"Recursive for directories. Refuses to overwrite an existing destination unless overwrite=\"true\". " +
			"Symlinks are not followed and will be rejected.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for copying a file or directory",
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
					Description: "Short user-facing status shown in the host UI while this tool runs. Frame it in the user's terms (e.g., 'making a copy of the report'). Separate channel from your chat reply — don't repeat it there.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"source", "destination", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the copy",
			Properties: map[string]*ai.Schema{
				"success": {Type: ai.TypeBoolean, Description: "Whether the copy succeeded"},
				"results": {Type: ai.TypeString, Description: "Description of what was copied"},
				"error":   {Type: ai.TypeString, Description: "Error message if the copy failed"},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for the cp tool.
func (c *CpTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if c.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				c.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "copyFile",
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

		if err := copyPath(resolvedSrc, resolvedDst, overwrite); err != nil {
			return failResult(err.Error()), nil
		}

		return map[string]any{
			"success": true,
			"results": fmt.Sprintf("copied %s to %s", source, destination),
		}, nil
	}
}

// copyPath copies src to dst. Refuses to follow symlinks. Refuses to
// overwrite unless overwrite is true. Recursive for directories.
func copyPath(src, dst string, overwrite bool) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("source %q: %w", src, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source %q is a symlink; refusing to copy", src)
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

	if info.IsDir() {
		return copyDir(src, dst, info.Mode().Perm())
	}
	return copyFile(src, dst, info.Mode().Perm())
}

func copyDir(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(dst, mode); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		info, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("entry %q is a symlink; refusing to copy", srcPath)
		}
		if info.IsDir() {
			if err := copyDir(srcPath, dstPath, info.Mode().Perm()); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath, info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// FormatOutput formats the cp result for user display.
func (c *CpTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**Copy failed**: %s", msg)
		}
		return "**Copy failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return msg
	}
	return "Copied."
}

func failResult(msg string) map[string]any {
	return map[string]any{
		"success": false,
		"results": "",
		"error":   msg,
	}
}
