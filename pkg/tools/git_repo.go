// Package tools — git tooling.
//
// Genie ships a set of workspace-restricted git tools that operate on
// the agent's filesystem via go-git (no shell-out, no `git` binary on
// the pod). The repo layout is the host's call: genie just resolves
// "the active repo" by walking up from the caller's cwd (or an
// explicit workspace-relative `repo` parameter) until it finds a
// `.git`. This honours git's normal "innermost wins" semantics.
//
// Author identity for commits is opaque to genie: the host sets
// `commit_author_name` and `commit_author_email` on the context per
// turn; the git tool writes those values verbatim. Genie does not
// interpret what they mean (user id, conversation id, owner id, etc.).
package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

// ErrNoRepo is returned when no .git can be found by walking up from
// the caller's resolved path. Callers should treat this as a
// "platform should have initialised the repo" condition and surface a
// clear error to the model.
var ErrNoRepo = errors.New("no git repository found")

// resolveRepoPath returns the absolute path of the active repo for a
// tool call. It honours an explicit `repo` parameter when provided
// (workspace-relative path inside the workspace); otherwise it walks
// up from the caller's cwd. The returned path is the directory that
// contains `.git/` — open it with go-git's PlainOpen.
//
// The returned path is always inside the workspace (or an allowed
// directory) — the resolver is invoked first so symlink components
// and outside-workspace paths are rejected by the same code that
// gates every other fileops tool.
func resolveRepoPath(ctx context.Context, repoParam string) (string, error) {
	startInput := repoParam
	if startInput == "" {
		// Default: caller's cwd.
		startInput = "."
	}

	resolved, valid := ResolvePathWithWorkingDirectory(ctx, startInput)
	if !valid {
		return "", FormatPathOutsideWorkspaceError(ctx, startInput)
	}

	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}

	// Walk up to find a .git directory or file (worktrees use a
	// `.git` file pointing at gitdir; go-git handles both).
	cur := abs
	for {
		if info, err := os.Stat(cur); err == nil && info.IsDir() {
			gitPath := filepath.Join(cur, ".git")
			if _, err := os.Lstat(gitPath); err == nil {
				return cur, nil
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("%w (searched up from %q)", ErrNoRepo, abs)
		}
		// Don't walk past the workspace boundary — even if a parent
		// dir happens to contain a .git, it's not part of the agent's
		// world.
		if !pathIsInsideAllowedRoots(ctx, parent) {
			return "", fmt.Errorf("%w (searched up from %q, did not find one inside the workspace)", ErrNoRepo, abs)
		}
		cur = parent
	}
}

// pathIsInsideAllowedRoots returns true if path is inside the cwd or
// any allowed_dir from context. Used by the repo walker to bound the
// upward search at the workspace boundary.
func pathIsInsideAllowedRoots(ctx context.Context, path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	wsAbs, err := filepath.Abs(WorkingDirectoryFromContext(ctx))
	if err == nil {
		if isWithinDir(abs, wsAbs) || abs == wsAbs {
			return true
		}
	}
	for _, allowed := range AllowedDirsFromContext(ctx) {
		if allowedAbs, err := filepath.Abs(allowed); err == nil {
			if isWithinDir(abs, allowedAbs) || abs == allowedAbs {
				return true
			}
		}
	}
	return false
}

// openRepo opens the active repo. Returns ErrNoRepo when none is
// found (caller should map to a clean tool error).
func openRepo(ctx context.Context, repoParam string) (*git.Repository, string, error) {
	path, err := resolveRepoPath(ctx, repoParam)
	if err != nil {
		return nil, "", err
	}
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, "", fmt.Errorf("open repo at %s: %w", path, err)
	}
	return repo, path, nil
}

// AuthorFromContext reads the opaque commit-author identity the host
// set on the context. If neither name nor email is set, returns a
// fallback so commits remain attributable rather than silently using
// the system git config.
func AuthorFromContext(ctx context.Context) (name, email string) {
	if v := ctx.Value("commit_author_name"); v != nil {
		if s, ok := v.(string); ok {
			name = s
		}
	}
	if v := ctx.Value("commit_author_email"); v != nil {
		if s, ok := v.(string); ok {
			email = s
		}
	}
	if name == "" {
		name = "mutiro-agent"
	}
	if email == "" {
		email = "noreply@mutiro.local"
	}
	return name, email
}

// repoRelative converts an absolute path to a path relative to the
// repo root, suitable for go-git APIs that expect repo-relative paths.
// Returns ("", false) if the path is outside the repo.
func repoRelative(repoRoot, absPath string) (string, bool) {
	rel, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return "", false
	}
	if rel == ".." || filepath.IsAbs(rel) || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
