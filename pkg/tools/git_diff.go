package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// GitDiffTool shows changes either in the working tree or for a
// specific commit.
type GitDiffTool struct{ publisher events.Publisher }

// NewGitDiffTool constructs the tool.
func NewGitDiffTool(publisher events.Publisher) Tool {
	return &GitDiffTool{publisher: publisher}
}

const gitDiffMaxLines = 2000

// Declaration returns the function declaration for gitDiff.
func (g *GitDiffTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "gitDiff",
		Description: "Show changes as a unified diff. Without `commit`, " +
			"shows working-tree changes against HEAD (what gitCommit " +
			"would record). With `commit`, shows what that commit " +
			"introduced compared to its parent. Output is truncated " +
			"past 2000 lines.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for gitDiff",
			Properties: map[string]*ai.Schema{
				"repo": {
					Type:        ai.TypeString,
					Description: "Optional workspace-relative path of the repo to query.",
					MaxLength:   500,
				},
				"commit": {
					Type:        ai.TypeString,
					Description: "Optional commit reference (sha, 'HEAD', 'HEAD~1'). When set, shows the diff this commit introduced.",
					MaxLength:   100,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status (e.g. 'showing the pending changes').",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"_display_message"},
		},
		Response: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"success":   {Type: ai.TypeBoolean},
				"results":   {Type: ai.TypeString, Description: "Unified diff or status summary"},
				"truncated": {Type: ai.TypeBoolean},
				"error":     {Type: ai.TypeString},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for gitDiff.
func (g *GitDiffTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if g.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				g.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "gitDiff",
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
		if err := CheckPathPolicy(ctx, repoPath, IntentRead); err != nil {
			return failResult(err.Error()), nil
		}

		commitRef, _ := params["commit"].(string)

		var diffText string
		if commitRef == "" {
			diffText, err = diffWorkingTree(repo)
		} else {
			diffText, err = diffCommit(repo, commitRef)
		}
		if err != nil {
			return failResult(err.Error()), nil
		}

		truncated := false
		lines := strings.Split(diffText, "\n")
		if len(lines) > gitDiffMaxLines {
			lines = lines[:gitDiffMaxLines]
			lines = append(lines, "... (truncated; pass a commit ref or narrow the path scope)")
			diffText = strings.Join(lines, "\n")
			truncated = true
		}

		return map[string]any{
			"success":   true,
			"results":   diffText,
			"truncated": truncated,
		}, nil
	}
}

// FormatOutput formats the diff for the host UI.
func (g *GitDiffTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**git diff failed**: %s", msg)
		}
		return "**git diff failed**"
	}
	diff, _ := result["results"].(string)
	if strings.TrimSpace(diff) == "" {
		return "**git diff**: no changes"
	}
	return fmt.Sprintf("**git diff**\n```diff\n%s\n```", strings.TrimRight(diff, "\n"))
}

// diffWorkingTree returns a status-style summary of pending changes.
// go-git does not expose a direct "diff worktree against HEAD" patch,
// and synthesising one would require reading each modified file plus
// the corresponding blob from HEAD — which we already do via
// readFile / gitShow. The summary here is the load-bearing answer to
// "what would gitCommit record?"; full content goes via the other
// tools.
func diffWorkingTree(repo *git.Repository) (string, error) {
	if _, err := repo.Head(); err != nil {
		return "(no commits yet — use gitStatus to see untracked files)", nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("status: %w", err)
	}
	if st.IsClean() {
		return "", nil
	}
	var b strings.Builder
	for path, fs := range st {
		fmt.Fprintf(&b, "%s%s %s\n", string(fs.Staging), string(fs.Worktree), path)
	}
	return b.String(), nil
}

// diffCommit returns the unified patch a specific commit introduced
// (against its first parent). Initial commits are reported as such
// since there's no parent to diff against.
func diffCommit(repo *git.Repository, ref string) (string, error) {
	hash, err := resolveRef(repo, ref)
	if err != nil {
		return "", err
	}
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return "", fmt.Errorf("load commit %s: %w", ref, err)
	}
	if commit.NumParents() == 0 {
		return "(initial commit; no parent to diff against)", nil
	}
	parent, err := commit.Parent(0)
	if err != nil {
		return "", fmt.Errorf("load parent of %s: %w", ref, err)
	}
	patch, err := parent.Patch(commit)
	if err != nil {
		return "", fmt.Errorf("compute patch: %w", err)
	}
	return patch.String(), nil
}

// resolveRef returns the hash for a ref string. Accepts full or short
// sha and "HEAD"/"HEAD~N" forms via go-git's revision parser.
func resolveRef(repo *git.Repository, ref string) (plumbing.Hash, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("resolve %q: %w", ref, err)
	}
	return *hash, nil
}
