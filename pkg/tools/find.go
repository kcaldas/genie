package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// FindTool finds files and directories within the workspace.
type FindTool struct {
	publisher events.Publisher
}

// NewFindTool creates a new find tool.
func NewFindTool(publisher events.Publisher) Tool {
	return &FindTool{
		publisher: publisher,
	}
}

// findFilesMaxResults caps the number of paths returned in one call.
// Beyond this the model should narrow its query.
const findFilesMaxResults = 1000

// Declaration returns the function declaration for the find tool.
func (f *FindTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "findFiles",
		Description: "Find files and directories matching a glob pattern. " +
			"Standard glob semantics: `*` matches within a directory component, " +
			"`**` matches across directory boundaries. " +
			"A pattern with no slash is a basename glob and matches at any " +
			"depth — `*.go` finds every Go file anywhere in the tree. " +
			"A pattern with a slash is anchored to the search root: " +
			"`src/*.go` matches only direct children of src/, " +
			"`src/**/*.go` matches every Go file at any depth under src/. " +
			"Symlinks are not followed. " +
			"Returns workspace-relative paths, sorted, capped at 1000 results " +
			"with a clear truncation note when there are more.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for finding files",
			Properties: map[string]*ai.Schema{
				"pattern": {
					Type: ai.TypeString,
					Description: "Glob pattern to match. Examples: " +
						"`*.go` (every Go file at any depth), " +
						"`**/*_test.go` (every test file), " +
						"`pkg/*.go` (only direct children of pkg/), " +
						"`pkg/**/*.go` (every Go file under pkg/), " +
						"`*test*` (any name containing 'test'), " +
						"`config.*` (config with any extension).",
					MinLength: 1,
					MaxLength: 200,
				},
				"path": {
					Type:        ai.TypeString,
					Description: "Optional starting directory, relative to the workspace. Defaults to the workspace root.",
					MaxLength:   500,
				},
				"type": {
					Type:        ai.TypeString,
					Description: "Type of items to return: 'file', 'directory', or 'any' (default).",
					Enum:        []string{"file", "directory", "any"},
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status shown in the host UI while this tool runs. Frame it in the user's terms (e.g., 'looking for the file you mentioned'). Separate channel from your chat reply — don't repeat it there.",
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
				"success":   {Type: ai.TypeBoolean, Description: "Whether the search was successful"},
				"results":   {Type: ai.TypeString, Description: "Newline-separated list of matching paths (workspace-relative)"},
				"truncated": {Type: ai.TypeBoolean, Description: "True when more matches existed than the result cap"},
				"error":     {Type: ai.TypeString, Description: "Error message if the search failed"},
			},
			Required: []string{"success", "results"},
		},
	}
}

// Handler returns the function handler for the find tool.
func (f *FindTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if f.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				f.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "findFiles",
					Message:  msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		pattern, ok := params["pattern"].(string)
		if !ok || pattern == "" {
			return nil, fmt.Errorf("pattern parameter is required and must be a non-empty string")
		}

		startPath := "."
		if v, ok := params["path"].(string); ok && v != "" {
			startPath = v
		}

		typeFilter := "any"
		if v, ok := params["type"].(string); ok {
			switch v {
			case "file", "directory", "any":
				typeFilter = v
			}
		}

		resolvedStart, isValid := ResolvePathWithWorkingDirectory(ctx, startPath)
		if !isValid {
			return nil, FormatPathOutsideWorkspaceError(ctx, startPath)
		}
		if err := CheckPathPolicy(ctx, resolvedStart, IntentRead); err != nil {
			return nil, err
		}

		workspace := WorkingDirectoryFromContext(ctx)
		absWorkspace, err := filepath.Abs(workspace)
		if err != nil {
			return nil, fmt.Errorf("resolve workspace: %w", err)
		}
		absStart, err := filepath.Abs(resolvedStart)
		if err != nil {
			return nil, fmt.Errorf("resolve start path: %w", err)
		}

		// Walk the tree. WalkDir does not follow symlinks, which matches
		// the workspace-restricted policy.
		var matches []string
		truncated := false
		walkErr := filepath.WalkDir(absStart, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				// A permission error or transient stat error on a single
				// entry shouldn't kill the whole walk. Skip and continue.
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			// Skip the start path itself when it's a directory — find
			// historically didn't list it as a match for a pattern.
			if p == absStart {
				return nil
			}
			// Defense in depth: even though WalkDir follows
			// dir-symlinks-into-the-tree only when explicitly told to,
			// reject any entry whose Lstat says symlink so the agent
			// never sees one in results.
			info, lstatErr := os.Lstat(p)
			if lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			rel, relErr := filepath.Rel(absWorkspace, p)
			if relErr != nil {
				return nil
			}
			// Normalize on forward slashes so glob patterns work the
			// same way Unix-style users write them, even on Windows.
			relForward := filepath.ToSlash(rel)

			// Honour the policy: denied paths are silently filtered.
			// We don't want listing to leak the existence of paths the
			// agent can't otherwise touch.
			if err := CheckPathPolicy(ctx, p, IntentRead); err != nil {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			// Type filter
			isDir := d.IsDir()
			switch typeFilter {
			case "file":
				if isDir {
					return nil
				}
			case "directory":
				if !isDir {
					return nil
				}
			}

			if !MatchGlob(pattern, relForward) {
				return nil
			}

			if isDir {
				relForward += "/"
			}
			matches = append(matches, relForward)

			if len(matches) >= findFilesMaxResults {
				truncated = true
				return fs.SkipAll
			}
			return nil
		})
		if walkErr != nil {
			return map[string]any{
				"success": false,
				"results": "",
				"error":   fmt.Sprintf("walk failed: %v", walkErr),
			}, nil
		}

		sort.Strings(matches)

		out := strings.Join(matches, "\n")
		result := map[string]any{
			"success":   true,
			"results":   out,
			"truncated": truncated,
		}
		if truncated {
			result["results"] = out + "\n... (truncated; narrow the pattern or path to see more)"
		}
		return result, nil
	}
}

// FormatOutput formats find results for user display.
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

	resultList := strings.Split(results, "\n")
	var formattedResults []string
	for _, item := range resultList {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasSuffix(item, "/") {
			formattedResults = append(formattedResults, "[DIR]  "+item)
		} else {
			formattedResults = append(formattedResults, "[FILE] "+item)
		}
	}
	return fmt.Sprintf("**Search Results**\n%s", strings.Join(formattedResults, "\n"))
}
