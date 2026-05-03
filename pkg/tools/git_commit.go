package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// GitCommitTool commits dirty files to the active repo, attributed to
// the author identity the host set on the context.
type GitCommitTool struct{ publisher events.Publisher }

// NewGitCommitTool constructs the tool.
func NewGitCommitTool(publisher events.Publisher) Tool {
	return &GitCommitTool{publisher: publisher}
}

// Declaration returns the function declaration for gitCommit.
func (g *GitCommitTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "gitCommit",
		Description: "Commit dirty files in the active repository with " +
			"the given message. By default commits everything dirty in " +
			"the repo. Pass `paths` to commit only specific files. The " +
			"author is set by the host (the platform-attributed actor " +
			"for this turn) — agents do not control authorship. Refuses " +
			"to span repos: every path in a single call must belong to " +
			"the same repo.",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"message": {
					Type:        ai.TypeString,
					Description: "Commit message. The first line is the subject; lines after a blank line form the body.",
					MinLength:   1,
					MaxLength:   2000,
				},
				"paths": {
					Type:        ai.TypeArray,
					Description: "Optional list of workspace-relative paths to commit. When omitted, commits every dirty file in the active repo.",
					Items:       &ai.Schema{Type: ai.TypeString, MaxLength: 500},
				},
				"repo": {
					Type:        ai.TypeString,
					Description: "Optional workspace-relative path of the repo to commit into. Inferred from cwd or paths otherwise.",
					MaxLength:   500,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status (e.g. 'committing the day's notes').",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"message", "_display_message"},
		},
		Response: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"success": {Type: ai.TypeBoolean},
				"results": {Type: ai.TypeString, Description: "Summary of the commit"},
				"sha":     {Type: ai.TypeString, Description: "Commit sha"},
				"error":   {Type: ai.TypeString},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for gitCommit.
func (g *GitCommitTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if g.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				g.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "gitCommit",
					Message:  msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		message, ok := params["message"].(string)
		if !ok || strings.TrimSpace(message) == "" {
			return failResult("message parameter is required and must be non-empty"), nil
		}

		// Optional explicit path list. If provided, every path must be
		// in the workspace and pass the mutate policy.
		var explicitPaths []string
		if v, ok := params["paths"]; ok {
			switch t := v.(type) {
			case []any:
				for _, x := range t {
					if s, ok := x.(string); ok && s != "" {
						explicitPaths = append(explicitPaths, s)
					}
				}
			case []string:
				explicitPaths = append(explicitPaths, t...)
			}
		}

		// Resolve the active repo. If `repo` is set, use it. Otherwise
		// derive from the first explicit path (if any) so the agent
		// can commit by saying "these files" rather than having to be
		// in the right cwd.
		repoParam, _ := params["repo"].(string)
		repoHint := repoParam
		if repoHint == "" && len(explicitPaths) > 0 {
			repoHint = explicitPaths[0]
		}
		repo, repoPath, err := openRepo(ctx, repoHint)
		if err != nil {
			return failResult(err.Error()), nil
		}
		if err := CheckPathPolicy(ctx, repoPath, IntentMutate); err != nil {
			return failResult(err.Error()), nil
		}

		wt, err := repo.Worktree()
		if err != nil {
			return failResult(fmt.Sprintf("worktree: %v", err)), nil
		}

		// Build the path set to commit. If explicit paths were given,
		// validate each lives in the same repo (refuse cross-repo
		// commits). If not, use every dirty file as discovered by
		// status — already implicitly single-repo since status is on
		// one repo.
		var pathsToAdd []string
		if len(explicitPaths) > 0 {
			pathsToAdd, err = collectExplicitCommitPaths(ctx, repoPath, explicitPaths)
			if err != nil {
				return failResult(err.Error()), nil
			}
		} else {
			st, err := wt.Status()
			if err != nil {
				return failResult(fmt.Sprintf("status: %v", err)), nil
			}
			if st.IsClean() {
				return failResult("nothing to commit — working tree is clean"), nil
			}
			for p := range st {
				pathsToAdd = append(pathsToAdd, p)
			}
		}
		sort.Strings(pathsToAdd)

		// Stage every path. AddWithOptions handles deletes too.
		for _, p := range pathsToAdd {
			if err := wt.AddWithOptions(&git.AddOptions{Path: p}); err != nil {
				return failResult(fmt.Sprintf("stage %s: %v", p, err)), nil
			}
		}

		// Author from context. Mutiro decides what these mean.
		authorName, authorEmail := AuthorFromContext(ctx)
		hash, err := wt.Commit(message, &git.CommitOptions{
			Author: &object.Signature{
				Name:  authorName,
				Email: authorEmail,
				When:  time.Now(),
			},
		})
		if err != nil {
			return failResult(fmt.Sprintf("commit: %v", err)), nil
		}

		short := hash.String()
		if len(short) > 12 {
			short = short[:12]
		}

		return map[string]any{
			"success": true,
			"results": fmt.Sprintf("committed %s as %s <%s>: %s",
				short, authorName, authorEmail, firstLine(message)),
			"sha": hash.String(),
		}, nil
	}
}

// FormatOutput formats the commit result for the host UI.
func (g *GitCommitTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**git commit failed**: %s", msg)
		}
		return "**git commit failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return fmt.Sprintf("**git commit**: %s", msg)
	}
	return "**git commit**: ok"
}

// collectExplicitCommitPaths validates each requested path against the
// workspace + policy, ensures it lives in the same repo as repoPath,
// and returns repo-relative paths suitable for AddWithOptions.
func collectExplicitCommitPaths(ctx context.Context, repoPath string, paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		resolved, valid := ResolvePathWithWorkingDirectory(ctx, p)
		if !valid {
			return nil, FormatPathOutsideWorkspaceError(ctx, p)
		}
		if err := CheckPathPolicy(ctx, resolved, IntentMutate); err != nil {
			return nil, err
		}
		abs, err := filepath.Abs(resolved)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", p, err)
		}
		rel, ok := repoRelative(repoPath, abs)
		if !ok {
			return nil, fmt.Errorf("path %q is outside the active repo at %s; gitCommit refuses to span repos", p, repoPath)
		}
		out = append(out, rel)
	}
	return out, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
