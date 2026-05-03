package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// MaxEditFileSize caps the size of files editFile will read. Beyond
// this, the model should be using a different strategy (write a new
// file, work on a slice, etc.) — and we don't want to OOM the daemon.
const MaxEditFileSize int64 = 10 * 1024 * 1024 // 10 MiB

// EditTool edits a file inside the workspace using either string-based
// search-and-replace or line-range replacement.
type EditTool struct {
	publisher events.Publisher
}

// NewEditTool creates a new edit tool.
func NewEditTool(publisher events.Publisher) Tool {
	return &EditTool{publisher: publisher}
}

// Declaration returns the function declaration for the edit tool.
func (e *EditTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "editFile",
		Description: "Edit a file inside the workspace. Two modes — pick one:\n\n" +
			"  (1) String replace: provide old_string and new_string. " +
			"old_string must match EXACTLY ONCE in the file. If it appears " +
			"zero times the edit fails (file may have changed). If it " +
			"appears more than once the edit fails — add surrounding " +
			"context to make it unique. This safety property is the whole " +
			"point: if the match isn't unique, you can't be sure which " +
			"instance you meant to edit.\n\n" +
			"  (2) Line range: provide start_line, end_line, and " +
			"replacement. Both line numbers are 1-indexed and inclusive. " +
			"replacement is the new content for that range (omit the " +
			"replacement parameter to delete the lines).\n\n" +
			"Modes are mutually exclusive — provide one set of parameters " +
			"or the other. The tool writes via temp-file + atomic rename, " +
			"so concurrent readers never see a partial state.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for editing a file",
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "File path to edit (relative to the workspace, or absolute inside the workspace). The file must exist.",
					MinLength:   1,
					MaxLength:   500,
				},
				"old_string": {
					Type:        ai.TypeString,
					Description: "(str_replace mode) Exact string to find. Must occur EXACTLY ONCE in the file.",
				},
				"new_string": {
					Type:        ai.TypeString,
					Description: "(str_replace mode) Replacement for old_string. Pass the empty string to delete the matched span.",
				},
				"start_line": {
					Type:        ai.TypeInteger,
					Description: "(line-range mode) First line of the range to replace, 1-indexed inclusive.",
					Minimum:     1,
				},
				"end_line": {
					Type:        ai.TypeInteger,
					Description: "(line-range mode) Last line of the range to replace, 1-indexed inclusive.",
					Minimum:     1,
				},
				"replacement": {
					Type:        ai.TypeString,
					Description: "(line-range mode) New content for the line range. Use an empty string to delete the lines. Trailing newline is added automatically if missing.",
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status shown in the host UI while this tool runs. Frame it in the user's terms (e.g., 'updating the README intro'). Separate channel from your chat reply — don't repeat it there.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"path", "_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the edit",
			Properties: map[string]*ai.Schema{
				"success":   {Type: ai.TypeBoolean, Description: "Whether the edit succeeded"},
				"results":   {Type: ai.TypeString, Description: "Human-readable summary of the edit"},
				"file_size": {Type: ai.TypeInteger, Description: "Size of the file in bytes after the edit"},
				"error":     {Type: ai.TypeString, Description: "Error message if the edit failed"},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for the edit tool.
func (e *EditTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if e.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				e.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "editFile",
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

		// Mode discrimination based on which parameters are present.
		mode, modeErr := pickEditMode(params)
		if modeErr != nil {
			return failResult(modeErr.Error()), nil
		}

		resolved, valid := ResolvePathWithWorkingDirectory(ctx, path)
		if !valid {
			return failResult(FormatPathOutsideWorkspaceError(ctx, path).Error()), nil
		}
		if err := CheckPathPolicy(ctx, resolved, IntentMutate); err != nil {
			return failResult(err.Error()), nil
		}

		// Source must exist and be a regular file.
		info, err := os.Lstat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				return failResult(fmt.Sprintf("path %q does not exist", path)), nil
			}
			return failResult(fmt.Sprintf("stat %q: %v", path, err)), nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return failResult(fmt.Sprintf("path %q is a symlink; refusing to edit", path)), nil
		}
		if info.IsDir() {
			return failResult(fmt.Sprintf("path %q is a directory, not a file", path)), nil
		}
		if info.Size() > MaxEditFileSize {
			return failResult(fmt.Sprintf(
				"file %q is %d bytes which exceeds the editFile cap of %d; "+
					"use writeFile to replace it whole, or operate on a slice",
				path, info.Size(), MaxEditFileSize,
			)), nil
		}

		original, err := os.ReadFile(resolved)
		if err != nil {
			return failResult(fmt.Sprintf("read file: %v", err)), nil
		}

		var (
			updated []byte
			summary string
		)
		switch mode {
		case editModeStrReplace:
			updated, summary, err = applyStrReplace(original, params)
		case editModeLineRange:
			updated, summary, err = applyLineRange(original, params)
		default:
			err = fmt.Errorf("unknown edit mode")
		}
		if err != nil {
			return failResult(err.Error()), nil
		}

		if err := atomicWriteFile(resolved, updated, info.Mode().Perm()); err != nil {
			return failResult(fmt.Sprintf("write file: %v", err)), nil
		}

		return map[string]any{
			"success":   true,
			"results":   summary,
			"file_size": int64(len(updated)),
		}, nil
	}
}

// FormatOutput formats the edit result for user display.
func (e *EditTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**Edit failed**: %s", msg)
		}
		return "**Edit failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return msg
	}
	return "Edit applied."
}

// editMode discriminates the two editFile call shapes.
type editMode int

const (
	editModeUnspecified editMode = iota
	editModeStrReplace
	editModeLineRange
)

// pickEditMode determines which mode the caller asked for, returning an
// error when the parameters are ambiguous, mixed, or incomplete.
func pickEditMode(params map[string]any) (editMode, error) {
	hasOld := hasNonEmptyString(params, "old_string")
	hasNew := hasString(params, "new_string")
	hasStart := hasNumber(params, "start_line")
	hasEnd := hasNumber(params, "end_line")
	hasReplacement := hasString(params, "replacement")

	strMode := hasOld || hasNew
	lineMode := hasStart || hasEnd || hasReplacement

	if strMode && lineMode {
		return editModeUnspecified, fmt.Errorf(
			"editFile: provide either (old_string + new_string) for str_replace mode, " +
				"OR (start_line + end_line + replacement) for line-range mode — not both")
	}
	if !strMode && !lineMode {
		return editModeUnspecified, fmt.Errorf(
			"editFile: missing edit parameters. Provide either (old_string + new_string) " +
				"or (start_line + end_line + replacement)")
	}
	if strMode {
		if !hasOld {
			return editModeUnspecified, fmt.Errorf("editFile str_replace mode requires old_string")
		}
		if !hasNew {
			return editModeUnspecified, fmt.Errorf(
				"editFile str_replace mode requires new_string (use the empty string to delete the match)")
		}
		return editModeStrReplace, nil
	}
	// line range mode
	if !hasStart || !hasEnd {
		return editModeUnspecified, fmt.Errorf("editFile line-range mode requires both start_line and end_line")
	}
	return editModeLineRange, nil
}

func applyStrReplace(original []byte, params map[string]any) ([]byte, string, error) {
	old, _ := params["old_string"].(string)
	repl, _ := params["new_string"].(string)
	if old == "" {
		return nil, "", fmt.Errorf("old_string must be a non-empty string")
	}

	count := strings.Count(string(original), old)
	switch count {
	case 0:
		return nil, "", fmt.Errorf(
			"old_string was not found in the file. The file may have changed since you last read it; re-read it and try again")
	case 1:
		// happy path
	default:
		return nil, "", fmt.Errorf(
			"old_string matches %d places in the file; add more surrounding context until it is unique. "+
				"This is a safety check — if the match is not unique, the wrong instance might be edited", count)
	}

	updated := strings.Replace(string(original), old, repl, 1)
	return []byte(updated), fmt.Sprintf("replaced %d-byte span with %d-byte content", len(old), len(repl)), nil
}

func applyLineRange(original []byte, params map[string]any) ([]byte, string, error) {
	start, ok := numberValue(params, "start_line")
	if !ok {
		return nil, "", fmt.Errorf("start_line is required")
	}
	end, ok := numberValue(params, "end_line")
	if !ok {
		return nil, "", fmt.Errorf("end_line is required")
	}
	if start < 1 {
		return nil, "", fmt.Errorf("start_line must be >= 1 (got %d)", start)
	}
	if end < start {
		return nil, "", fmt.Errorf("end_line (%d) must be >= start_line (%d)", end, start)
	}
	repl, _ := params["replacement"].(string)

	// Split into lines, preserving line endings semantically — we
	// reconstruct with '\n' and add a trailing newline only if the
	// replacement doesn't end with one (matches what humans expect when
	// editing with sed/vim).
	lines := splitLinesKeepEmpty(original)
	if int(end) > len(lines) {
		return nil, "", fmt.Errorf("end_line %d exceeds file length (%d lines)", end, len(lines))
	}

	before := lines[:start-1]
	after := lines[end:]

	var middle []string
	if repl != "" {
		// Normalise: drop one trailing newline if present, then split.
		// We re-insert the newline in the join so the file shape is
		// consistent.
		trimmed := strings.TrimSuffix(repl, "\n")
		middle = strings.Split(trimmed, "\n")
	}

	merged := make([]string, 0, len(before)+len(middle)+len(after))
	merged = append(merged, before...)
	merged = append(merged, middle...)
	merged = append(merged, after...)

	out := strings.Join(merged, "\n")
	// Preserve trailing newline if the original had one.
	if len(original) > 0 && original[len(original)-1] == '\n' {
		out += "\n"
	}
	summary := fmt.Sprintf("replaced lines %d–%d (%d lines) with %d lines",
		start, end, end-start+1, len(middle))
	return []byte(out), summary, nil
}

// splitLinesKeepEmpty splits a buffer into lines without dropping trailing
// empty lines or eating the final newline. Using strings.Split on "\n"
// gives us 1-based line indexing that matches editor conventions.
func splitLinesKeepEmpty(b []byte) []string {
	s := string(b)
	if s == "" {
		return nil
	}
	// Trim a single trailing newline so "a\nb\n" → ["a","b"], not
	// ["a","b",""]. start_line=2 then refers to "b" as humans expect.
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// atomicWriteFile writes data to dst via a temp file in the same directory
// and a rename. Concurrent readers either see the previous content or the
// new content — never a partial write.
func atomicWriteFile(dst string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(dst)+".edit-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := io.Copy(tmp, strings.NewReader(string(data))); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		cleanup()
		return fmt.Errorf("rename temp file into place: %w", err)
	}
	return nil
}

// ===== small param helpers =====

func hasString(params map[string]any, key string) bool {
	_, ok := params[key].(string)
	return ok
}
func hasNonEmptyString(params map[string]any, key string) bool {
	s, ok := params[key].(string)
	return ok && s != ""
}
func hasNumber(params map[string]any, key string) bool {
	_, ok := numberValue(params, key)
	return ok
}
func numberValue(params map[string]any, key string) (int64, bool) {
	switch v := params[key].(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}
