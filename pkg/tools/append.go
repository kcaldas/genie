package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// AppendTool appends content to a file inside the workspace.
type AppendTool struct {
	publisher events.Publisher
}

// NewAppendTool creates a new append tool.
func NewAppendTool(publisher events.Publisher) Tool {
	return &AppendTool{publisher: publisher}
}

// Declaration returns the function declaration for the append tool.
func (a *AppendTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "appendFile",
		Description: "Append content to a file inside the workspace. Creates " +
			"the file (and any missing parent directories) if it does not " +
			"exist. Use this for incremental notes, logs, and accumulating " +
			"output — it avoids the read-modify-write cycle that loses data " +
			"under concurrent edits and is far cheaper for large files than " +
			"writeFile. Refuses to operate on a path with any symlink " +
			"component.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for appending to a file",
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "File path to append to (relative to the workspace, or absolute inside the workspace)",
					MinLength:   1,
					MaxLength:   500,
				},
				"content": {
					Type:        ai.TypeString,
					Description: "Content to append. Include any newlines you want — the tool does not insert one for you.",
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status shown in the host UI while this tool runs. Frame it in the user's terms (e.g., 'jotting down the new finding'). Separate channel from your chat reply — don't repeat it there.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"path", "content", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the append",
			Properties: map[string]*ai.Schema{
				"success":   {Type: ai.TypeBoolean, Description: "Whether the append succeeded"},
				"results":   {Type: ai.TypeString, Description: "Description of what was done"},
				"file_size": {Type: ai.TypeInteger, Description: "Size of the file in bytes after the append"},
				"error":     {Type: ai.TypeString, Description: "Error message if the append failed"},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for the append tool.
func (a *AppendTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if a.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				a.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "appendFile",
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
		content, ok := params["content"].(string)
		if !ok {
			return failResult("content parameter is required and must be a string"), nil
		}

		resolved, valid := ResolvePathWithWorkingDirectory(ctx, path)
		if !valid {
			return failResult(FormatPathOutsideWorkspaceError(ctx, path).Error()), nil
		}
		if err := CheckPathPolicy(ctx, resolved, IntentMutate); err != nil {
			return failResult(err.Error()), nil
		}

		// If the leaf already exists, it must be a regular file. Lstat
		// catches a TOCTOU symlink swap on the leaf even though the
		// resolver already rejected symlink components.
		if info, err := os.Lstat(resolved); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return failResult(fmt.Sprintf("path %q is a symlink; refusing to append", path)), nil
			}
			if info.IsDir() {
				return failResult(fmt.Sprintf("path %q is a directory, not a file", path)), nil
			}
		} else if !os.IsNotExist(err) {
			return failResult(fmt.Sprintf("stat %q: %v", path, err)), nil
		}

		// Ensure parent directories exist (matches writeFile semantics).
		if err := ensureParent(resolved); err != nil {
			return failResult(err.Error()), nil
		}

		f, err := os.OpenFile(resolved, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return failResult(fmt.Sprintf("open file: %v", err)), nil
		}
		// O_APPEND makes write atomic at the OS level for buffers up to
		// PIPE_BUF (typically 4KB on Linux, 16KB on macOS). For larger
		// payloads concurrent writers may interleave — but the daemon is
		// single-tenant so the realistic exposure is zero.
		n, writeErr := f.WriteString(content)
		closeErr := f.Close()
		if writeErr != nil {
			return failResult(fmt.Sprintf("write: %v", writeErr)), nil
		}
		if closeErr != nil {
			return failResult(fmt.Sprintf("close: %v", closeErr)), nil
		}

		size := int64(-1)
		if info, err := os.Stat(resolved); err == nil {
			size = info.Size()
		}

		return map[string]any{
			"success":   true,
			"results":   fmt.Sprintf("appended %d bytes to %s", n, path),
			"file_size": size,
		}, nil
	}
}

// FormatOutput formats the append result for user display.
func (a *AppendTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**Append failed**: %s", msg)
		}
		return "**Append failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return msg
	}
	return "Appended."
}

// ensureParent creates the parent directory chain for path. Used by
// appendFile and editFile so they match writeFile's auto-create semantics.
func ensureParent(path string) error {
	parent := filepath.Dir(path)
	if parent == "" || parent == "." {
		return nil
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("ensure parent directory: %w", err)
	}
	return nil
}
