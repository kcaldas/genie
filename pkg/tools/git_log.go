package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// GitLogTool walks commit history of the active repo.
type GitLogTool struct{ publisher events.Publisher }

const (
	gitLogDefaultLimit = 20
	gitLogMaxLimit     = 100
)

// NewGitLogTool constructs the tool.
func NewGitLogTool(publisher events.Publisher) Tool {
	return &GitLogTool{publisher: publisher}
}

// Declaration returns the function declaration for gitLog.
func (g *GitLogTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "gitLog",
		Description: "Show the commit history of the active repository. " +
			"Returns commit sha, author, date, and message. Optionally " +
			"filter by `path` (only commits that touched that path).",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for gitLog",
			Properties: map[string]*ai.Schema{
				"repo": {
					Type:        ai.TypeString,
					Description: "Optional workspace-relative path of the repo to query.",
					MaxLength:   500,
				},
				"path": {
					Type:        ai.TypeString,
					Description: "Optional path filter (repo-relative or workspace-relative). Only commits that touched this path are returned.",
					MaxLength:   500,
				},
				"limit": {
					Type:        ai.TypeInteger,
					Description: fmt.Sprintf("Maximum commits to return (1–%d, default %d).", gitLogMaxLimit, gitLogDefaultLimit),
					Minimum:     1,
					Maximum:     gitLogMaxLimit,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Short user-facing status (e.g. 'pulling up the recent history').",
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
				"results": {Type: ai.TypeString, Description: "Human-readable commit list"},
				"count":   {Type: ai.TypeInteger, Description: "Number of commits returned"},
				"error":   {Type: ai.TypeString},
			},
			Required: []string{"success"},
		},
	}
}

// Handler returns the function handler for gitLog.
func (g *GitLogTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		if g.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				g.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "gitLog",
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

		limit := gitLogDefaultLimit
		if v, ok := numberValue(params, "limit"); ok {
			limit = int(v)
			if limit < 1 {
				limit = 1
			}
			if limit > gitLogMaxLimit {
				limit = gitLogMaxLimit
			}
		}

		var pathFilter string
		if v, ok := params["path"].(string); ok && v != "" {
			pathFilter = v
		}

		// If a path filter is given, normalise it to repo-relative.
		var pathRel string
		if pathFilter != "" {
			resolved, valid := ResolvePathWithWorkingDirectory(ctx, pathFilter)
			if !valid {
				return failResult(FormatPathOutsideWorkspaceError(ctx, pathFilter).Error()), nil
			}
			abs, err := filepath.Abs(resolved)
			if err != nil {
				return failResult(fmt.Sprintf("resolve path filter: %v", err)), nil
			}
			rel, ok := repoRelative(repoPath, abs)
			if !ok {
				return failResult(fmt.Sprintf("path %q is outside the active repo", pathFilter)), nil
			}
			pathRel = rel
		}

		head, err := repo.Head()
		if err != nil {
			// Empty repo — no commits yet. Return empty list cleanly.
			return map[string]any{
				"success": true,
				"results": "(no commits yet)",
				"count":   0,
			}, nil
		}

		opts := &git.LogOptions{From: head.Hash()}
		if pathRel != "" {
			opts.PathFilter = func(p string) bool { return p == pathRel }
		}
		iter, err := repo.Log(opts)
		if err != nil {
			return failResult(fmt.Sprintf("log: %v", err)), nil
		}
		defer iter.Close()

		var entries []string
		count := 0
		err = iter.ForEach(func(c *object.Commit) error {
			if count >= limit {
				return errStopIter
			}
			short := c.Hash.String()
			if len(short) > 12 {
				short = short[:12]
			}
			subject := c.Message
			if i := strings.IndexByte(subject, '\n'); i >= 0 {
				subject = subject[:i]
			}
			entries = append(entries,
				fmt.Sprintf("%s  %s <%s>  %s\n    %s",
					short,
					c.Author.Name,
					c.Author.Email,
					c.Author.When.Format("2006-01-02 15:04:05 -0700"),
					subject,
				))
			count++
			return nil
		})
		if err != nil && err != errStopIter {
			return failResult(fmt.Sprintf("walk log: %v", err)), nil
		}

		results := strings.Join(entries, "\n")
		if results == "" {
			results = "(no matching commits)"
		}
		return map[string]any{
			"success": true,
			"results": results,
			"count":   count,
		}, nil
	}
}

// FormatOutput formats the log result for the host UI.
func (g *GitLogTool) FormatOutput(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); !success {
		if msg, _ := result["error"].(string); msg != "" {
			return fmt.Sprintf("**git log failed**: %s", msg)
		}
		return "**git log failed**"
	}
	if msg, _ := result["results"].(string); msg != "" {
		return fmt.Sprintf("**git log**\n```\n%s\n```", strings.TrimRight(msg, "\n"))
	}
	return "**git log**: empty"
}

// errStopIter is a sentinel used to break out of go-git's ForEach
// without treating the early return as an error.
var errStopIter = fmt.Errorf("stop")
