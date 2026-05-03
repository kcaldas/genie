package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// GitStatusTool reports working-tree state of the active repo.
type GitStatusTool struct{ publisher events.Publisher }

// NewGitStatusTool constructs the tool.
func NewGitStatusTool(publisher events.Publisher) Tool {
	return &GitStatusTool{publisher: publisher}
}

// Declaration returns the function declaration for gitStatus.
func (g *GitStatusTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "gitStatus",
		Description: "Show the working-tree state of the active git " +
			"repository: which files are modified, added, deleted, " +
			"renamed, or untracked, plus the current branch and HEAD " +
			"sha. By default operates on the repo enclosing the " +
			"current workspace; pass `repo` to target a specific repo " +
			"(e.g. when querying a per-conversation repo from a " +
			"parent workspace).",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for gitStatus",
			Properties: map[string]*ai.Schema{
				"repo": {
					Type:        ai.TypeString,
					Description: "Optional workspace-relative path of the repo to query. Defaults to the repo enclosing the current cwd.",
					MaxLength:   500,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status (e.g. 'checking what's changed').",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"_display_message"},
		},
		Response: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"success": {Type: ai.TypeBoolean},
				"results": {Type: ai.TypeString, Description: "Human-readable status output"},
				"branch":  {Type: ai.TypeString, Description: "Current branch name"},
				"head":    {Type: ai.TypeString, Description: "HEAD commit short sha"},
				"clean":   {Type: ai.TypeBoolean, Description: "True when the working tree has no changes"},
				"error":   {Type: ai.TypeString},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for gitStatus.
func (g *GitStatusTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if g.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				g.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "gitStatus",
					Message:  msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		repoParam, _ := params["repo"].(string)
		repo, repoPath, err := openRepo(ctx, repoParam)
		if err != nil {
			return failResult(err.Error()), nil
		}
		// Read-intent policy check on the repo root.
		if err := CheckPathPolicy(ctx, repoPath, IntentRead); err != nil {
			return failResult(err.Error()), nil
		}

		branch, head := branchAndHead(repo)

		wt, err := repo.Worktree()
		if err != nil {
			return failResult(fmt.Sprintf("worktree: %v", err)), nil
		}
		status, err := wt.Status()
		if err != nil {
			return failResult(fmt.Sprintf("status: %v", err)), nil
		}

		clean := status.IsClean()

		paths := make([]string, 0, len(status))
		for p := range status {
			paths = append(paths, p)
		}
		sort.Strings(paths)

		var b strings.Builder
		fmt.Fprintf(&b, "branch: %s\n", branch)
		fmt.Fprintf(&b, "head:   %s\n", head)
		if clean {
			fmt.Fprintln(&b, "working tree clean")
		} else {
			fmt.Fprintln(&b, "changes:")
			for _, p := range paths {
				fs := status.File(p)
				fmt.Fprintf(&b, "  %s%s %s\n", string(fs.Staging), string(fs.Worktree), p)
			}
		}

		return map[string]any{
			"success": true,
			"results": b.String(),
			"branch":  branch,
			"head":    head,
			"clean":   clean,
		}, nil
	}
}

// FormatOutput returns a user-facing status panel.
func (g *GitStatusTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**git status failed**: %s", msg)
		}
		return "**git status failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return fmt.Sprintf("**git status**\n```\n%s\n```", strings.TrimRight(msg, "\n"))
	}
	return "**git status**: clean"
}

// branchAndHead returns the current branch (e.g. "main") and the
// short HEAD sha. Empty repo or detached HEAD are tolerated.
func branchAndHead(repo *git.Repository) (string, string) {
	branch := ""
	head := ""
	if h, err := repo.Head(); err == nil {
		head = h.Hash().String()
		if len(head) > 12 {
			head = head[:12]
		}
		if h.Name().IsBranch() {
			branch = h.Name().Short()
		} else {
			branch = "(detached)"
		}
	} else {
		branch = "(no commits yet)"
	}
	return branch, head
}
