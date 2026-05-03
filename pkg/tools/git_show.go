package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// GitShowTool reads a file's contents at a specific commit.
type GitShowTool struct{ publisher events.Publisher }

// NewGitShowTool constructs the tool.
func NewGitShowTool(publisher events.Publisher) Tool {
	return &GitShowTool{publisher: publisher}
}

const gitShowMaxFileSize = 5 * 1024 * 1024 // 5 MiB

// Declaration returns the function declaration for gitShow.
func (g *GitShowTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "gitShow",
		Description: "Read a file's contents at a specific commit. " +
			"Use this to inspect what something looked like in the " +
			"past, or to recover content the agent has since edited.",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "File path (workspace-relative or absolute inside the workspace).",
					MinLength:   1,
					MaxLength:   500,
				},
				"commit": {
					Type:        ai.TypeString,
					Description: "Commit reference: sha, 'HEAD', 'HEAD~1', etc.",
					MinLength:   1,
					MaxLength:   100,
				},
				"repo": {
					Type:        ai.TypeString,
					Description: "Optional workspace-relative path of the repo to query.",
					MaxLength:   500,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status (e.g. 'looking at the previous version').",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"path", "commit", "_display_message"},
		},
		Response: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"success": {Type: ai.TypeBoolean},
				"results": {Type: ai.TypeString, Description: "File contents at the specified commit"},
				"error":   {Type: ai.TypeString},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for gitShow.
func (g *GitShowTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if g.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				g.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "gitShow",
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
		commitRef, ok := params["commit"].(string)
		if !ok || commitRef == "" {
			return failResult("commit parameter is required"), nil
		}

		// Path must resolve to inside the workspace, and pass the
		// read policy. We don't need the file to exist on disk now —
		// we're reading from history — but the policy still applies
		// so denied paths can't be exhumed via gitShow.
		resolvedPath, valid := ResolvePathWithWorkingDirectory(ctx, path)
		if !valid {
			return failResult(FormatPathOutsideWorkspaceError(ctx, path).Error()), nil
		}
		if err := CheckPathPolicy(ctx, resolvedPath, IntentRead); err != nil {
			return failResult(err.Error()), nil
		}

		repoParam, _ := params["repo"].(string)
		repo, repoPath, err := openRepo(ctx, repoParam)
		if err != nil {
			return failResult(err.Error()), nil
		}
		if err := CheckPathPolicy(ctx, repoPath, IntentRead); err != nil {
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
		size := entry.Size
		if size > gitShowMaxFileSize {
			return failResult(fmt.Sprintf("file %q at %s is %d bytes, exceeds gitShow cap of %d", rel, commitRef, size, gitShowMaxFileSize)), nil
		}

		contents, err := entry.Contents()
		if err != nil {
			return failResult(fmt.Sprintf("read file: %v", err)), nil
		}

		return map[string]any{
			"success": true,
			"results": contents,
		}, nil
	}
}

// FormatOutput formats gitShow output for the host UI.
func (g *GitShowTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**git show failed**: %s", msg)
		}
		return "**git show failed**"
	}
	contents, _ := result["results"].(string)
	if contents == "" {
		return "**git show**: empty"
	}
	if len(contents) > 1500 {
		contents = contents[:1500] + "\n... (truncated for display)"
	}
	return fmt.Sprintf("**git show**\n```\n%s\n```", contents)
}
