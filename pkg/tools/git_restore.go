package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// GitRestoreTool restores a path to its content at HEAD or an explicit
// commit. The restore is atomic at the file level (temp + rename).
type GitRestoreTool struct{ publisher events.Publisher }

// NewGitRestoreTool constructs the tool.
func NewGitRestoreTool(publisher events.Publisher) Tool {
	return &GitRestoreTool{publisher: publisher}
}

// Declaration returns the function declaration for gitRestore.
func (g *GitRestoreTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "gitRestore",
		Description: "Restore a file's content from a previous commit. " +
			"Defaults to HEAD (undo unstaged edits). Pass `commit` to " +
			"reach further back. Writes are atomic. The restore is " +
			"NOT auto-committed — call gitCommit afterwards if you " +
			"want the restoration recorded.",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "File path to restore (workspace-relative or absolute inside the workspace).",
					MinLength:   1,
					MaxLength:   500,
				},
				"commit": {
					Type:        ai.TypeString,
					Description: "Commit reference to restore from. Defaults to 'HEAD'.",
					MaxLength:   100,
				},
				"repo": {
					Type:        ai.TypeString,
					Description: "Optional workspace-relative path of the repo to restore from.",
					MaxLength:   500,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status (e.g. 'rolling back the change').",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"path", "_display_message"},
		},
		Response: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"success": {Type: ai.TypeBoolean},
				"results": {Type: ai.TypeString, Description: "Summary of the restore"},
				"error":   {Type: ai.TypeString},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for gitRestore.
func (g *GitRestoreTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if g.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				g.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "gitRestore",
					Message:  msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		path, ok := params["path"].(string)
		if !ok || path == "" {
			return failResult("path parameter is required"), nil
		}

		commitRef := "HEAD"
		if v, ok := params["commit"].(string); ok && v != "" {
			commitRef = v
		}

		resolvedPath, valid := ResolvePathWithWorkingDirectory(ctx, path)
		if !valid {
			return failResult(FormatPathOutsideWorkspaceError(ctx, path).Error()), nil
		}
		if err := CheckPathPolicy(ctx, resolvedPath, IntentMutate); err != nil {
			return failResult(err.Error()), nil
		}

		repoParam, _ := params["repo"].(string)
		repoHint := repoParam
		if repoHint == "" {
			repoHint = path
		}
		repo, repoPath, err := openRepo(ctx, repoHint)
		if err != nil {
			return failResult(err.Error()), nil
		}
		if err := CheckPathPolicy(ctx, repoPath, IntentMutate); err != nil {
			return failResult(err.Error()), nil
		}

		absPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			return failResult(fmt.Sprintf("resolve path: %v", err)), nil
		}
		rel, ok := repoRelative(repoPath, absPath)
		if !ok {
			return failResult(fmt.Sprintf("path %q is outside the active repo", path)), nil
		}

		hash, err := resolveRef(repo, commitRef)
		if err != nil {
			return failResult(err.Error()), nil
		}
		commit, err := repo.CommitObject(hash)
		if err != nil {
			return failResult(fmt.Sprintf("load commit: %v", err)), nil
		}
		tree, err := commit.Tree()
		if err != nil {
			return failResult(fmt.Sprintf("load tree: %v", err)), nil
		}
		entry, err := tree.File(rel)
		if err != nil {
			return failResult(fmt.Sprintf("path %q not found at %s: %v", rel, commitRef, err)), nil
		}
		contents, err := entry.Contents()
		if err != nil {
			return failResult(fmt.Sprintf("read file from tree: %v", err)), nil
		}

		// Determine the mode to write with — preserve the existing
		// file's mode when there is one, otherwise default to 0644.
		mode := os.FileMode(0o644)
		if info, err := os.Lstat(resolvedPath); err == nil {
			mode = info.Mode().Perm()
		}

		if err := ensureParent(resolvedPath); err != nil {
			return failResult(err.Error()), nil
		}
		if err := atomicWriteFile(resolvedPath, []byte(contents), mode); err != nil {
			return failResult(fmt.Sprintf("write restored file: %v", err)), nil
		}

		return map[string]any{
			"success": true,
			"results": fmt.Sprintf("restored %s from %s (%d bytes) — call gitCommit to record this", path, commitRef, len(contents)),
		}, nil
	}
}

// FormatOutput formats the restore result for the host UI.
func (g *GitRestoreTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**git restore failed**: %s", msg)
		}
		return "**git restore failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return fmt.Sprintf("**git restore**: %s", msg)
	}
	return "**git restore**: ok"
}
