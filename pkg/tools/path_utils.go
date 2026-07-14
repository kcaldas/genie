package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kcaldas/genie/pkg/toolctx"
)

// WorkingDirectoryFromContext returns the workspace root the agent is bound
// to. Defaults to "." when the context carries no `cwd` value. Exported so
// tools can include it in error messages — the model needs to know what
// the workspace is to recover (e.g. retry with a path inside it).
func WorkingDirectoryFromContext(ctx context.Context) string {
	if s, ok := toolctx.WorkingDir(ctx); ok && s != "" {
		return s
	}
	return "."
}

// AllowedDirsFromContext returns the additional read-allowed directories
// the agent has been granted (besides the workspace root). Empty when none
// are configured.
func AllowedDirsFromContext(ctx context.Context) []string {
	return extractAllowedDirs(ctx)
}

// PathIntent describes what a tool wants to do with a resolved path. It
// drives the denied / read_only policy check: read-only paths permit
// IntentRead but reject IntentMutate; denied paths reject both.
type PathIntent string

const (
	// IntentRead covers read-only operations: list, find, read, grep,
	// view, and the source side of copyFile.
	IntentRead PathIntent = "read"
	// IntentMutate covers any operation that creates, overwrites,
	// renames, or deletes filesystem state.
	IntentMutate PathIntent = "mutate"
)

// DeniedPathsFromContext returns glob patterns the agent must not touch
// at all (read or mutate). Patterns match against paths relative to the
// workspace root, e.g. ".mutiro-agent.yaml" or ".git/**".
func DeniedPathsFromContext(ctx context.Context) []string {
	patterns, _ := toolctx.DeniedPaths(ctx)
	return patterns
}

// ReadOnlyPathsFromContext returns glob patterns the agent may read but
// not mutate. Same matching rules as DeniedPathsFromContext.
func ReadOnlyPathsFromContext(ctx context.Context) []string {
	patterns, _ := toolctx.ReadOnlyPaths(ctx)
	return patterns
}

// CheckPathPolicy returns an error suitable for forwarding to the model
// when the resolved path is not allowed for the given intent. Callers
// should invoke it after ResolvePathWithWorkingDirectory and before
// touching the filesystem.
//
//   - IntentRead is rejected when the path matches denied_paths.
//   - IntentMutate is rejected when the path matches denied_paths OR
//     read_only_paths.
//
// Pattern matching is location-aware. For a path inside the workspace,
// patterns match against the workspace-relative form (e.g. "secrets/**"
// against "secrets/foo.txt"). For a path inside an allowed_dir but
// outside the workspace — common for the "owner-managed shared folder"
// pattern, where the workspace is per-user but allowed_dirs grants
// access to a sibling — patterns also match against the
// allowed-dir-rooted form: basename(allowed_dir) + "/" + rel-to-dir.
// That lets a single pattern like "shared/**" express "everything in
// the shared allowed_dir" regardless of which user's workspace is
// active. Patterns with no slash are treated as basename globs and
// match at any depth (e.g. "*.yaml").
func CheckPathPolicy(ctx context.Context, resolvedPath string, intent PathIntent) error {
	candidates := policyMatchCandidates(ctx, resolvedPath)

	deniedPatterns := DeniedPathsFromContext(ctx)
	for _, c := range candidates {
		if matched, pattern := matchAny(c, deniedPatterns); matched {
			return fmt.Errorf("path %q is denied (matched %q)", resolvedPath, pattern)
		}
	}
	if intent == IntentMutate {
		readOnlyPatterns := ReadOnlyPathsFromContext(ctx)
		for _, c := range candidates {
			if matched, pattern := matchAny(c, readOnlyPatterns); matched {
				return fmt.Errorf("path %q is read-only (matched %q); the agent may read it but cannot modify it", resolvedPath, pattern)
			}
		}
	}
	return nil
}

// policyMatchCandidates returns every "logical" representation of
// resolvedPath that policy patterns may legitimately reference. A path
// gets at minimum its workspace-relative form; if it also falls inside an
// allowed_dir, the allowed-dir-rooted form (basename(dir) + rel-to-dir)
// is added as a second candidate so patterns can name allowed_dirs by
// their visible label rather than chasing per-user `../../` traversals.
func policyMatchCandidates(ctx context.Context, resolvedPath string) []string {
	candidates := []string{relativeToWorkspace(WorkingDirectoryFromContext(ctx), resolvedPath)}

	absTarget, err := filepath.Abs(resolvedPath)
	if err != nil {
		return candidates
	}
	for _, dir := range AllowedDirsFromContext(ctx) {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if !isWithinDir(absTarget, absDir) {
			continue
		}
		relToDir, err := filepath.Rel(absDir, absTarget)
		if err != nil || strings.HasPrefix(relToDir, "..") {
			continue
		}
		dirLabel := filepath.Base(absDir)
		if dirLabel == "" || dirLabel == "." || dirLabel == string(filepath.Separator) {
			continue
		}
		var labeled string
		if relToDir == "." {
			labeled = dirLabel
		} else {
			labeled = dirLabel + "/" + filepath.ToSlash(relToDir)
		}
		candidates = append(candidates, labeled)
	}
	return candidates
}

func relativeToWorkspace(workspace, target string) string {
	absWS, err := filepath.Abs(workspace)
	if err != nil {
		return target
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return target
	}
	rel, err := filepath.Rel(absWS, absTarget)
	if err != nil {
		return target
	}
	return rel
}

// MatchGlob reports whether path matches the glob pattern using standard
// glob semantics: `*` matches within a directory component, `**` matches
// across directory boundaries. A pattern with no `/` is treated as a
// basename glob and matches at any depth — `*.go` finds every Go file in
// the tree, the same as gitignore / ripgrep / fd.
//
// Path is the workspace-relative path (no leading slash). Returns false
// for malformed patterns rather than an error since callers typically
// can't recover from a bad pattern mid-walk.
func MatchGlob(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "/") {
		// Basename glob — match the leaf at any depth.
		matched, err := filepath.Match(pattern, filepath.Base(path))
		return err == nil && matched
	}
	segments := strings.Split(pattern, "/")
	parts := strings.Split(path, "/")
	return matchGlobSegments(segments, parts)
}

func matchGlobSegments(segments, parts []string) bool {
	if len(segments) == 0 {
		return len(parts) == 0
	}
	seg := segments[0]
	if seg == "**" {
		rest := segments[1:]
		if len(rest) == 0 {
			// Trailing ** matches everything remaining (including nothing).
			return true
		}
		// ** consumes 0+ components — try each split point.
		for i := 0; i <= len(parts); i++ {
			if matchGlobSegments(rest, parts[i:]) {
				return true
			}
		}
		return false
	}
	if len(parts) == 0 {
		return false
	}
	matched, err := filepath.Match(seg, parts[0])
	if err != nil || !matched {
		return false
	}
	return matchGlobSegments(segments[1:], parts[1:])
}

// matchAny returns whether path matches any of the glob patterns.
// Returns the matched pattern (for error messages) when true.
func matchAny(path string, patterns []string) (bool, string) {
	if len(patterns) == 0 {
		return false, ""
	}
	base := filepath.Base(path)
	for _, p := range patterns {
		// Direct match
		if p == path {
			return true, p
		}
		// `dir/**` prefix matches path == dir or anything under it
		if strings.HasSuffix(p, "/**") {
			prefix := strings.TrimSuffix(p, "/**")
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return true, p
			}
			continue
		}
		// `**/dir/...` is rewritten to a contains check
		if strings.HasPrefix(p, "**/") {
			suffix := strings.TrimPrefix(p, "**/")
			if path == suffix || strings.HasSuffix(path, "/"+suffix) {
				return true, p
			}
			continue
		}
		// filepath.Match against full path and against basename so that
		// "*.yaml" matches regardless of directory.
		if matched, err := filepath.Match(p, path); err == nil && matched {
			return true, p
		}
		if matched, err := filepath.Match(p, base); err == nil && matched {
			return true, p
		}
	}
	return false, ""
}

// FormatPathOutsideWorkspaceError returns a recoverable error message that
// names the offending input, the workspace root, and any allowed
// directories. Tools should use this in place of bare "path is outside
// working directory" so the model has enough information to retry with a
// valid path.
func FormatPathOutsideWorkspaceError(ctx context.Context, inputPath string) error {
	workspace := WorkingDirectoryFromContext(ctx)
	allowed := AllowedDirsFromContext(ctx)

	msg := fmt.Sprintf(
		"path %q is outside the workspace %q",
		inputPath, workspace,
	)
	if len(allowed) > 0 {
		msg += fmt.Sprintf(" and not in any allowed directory (%s)", strings.Join(allowed, ", "))
	}
	msg += ". Use a path relative to the workspace, or an absolute path inside the workspace"
	if len(allowed) > 0 {
		msg += " or one of the allowed directories"
	}
	msg += "."
	return fmt.Errorf("%s", msg)
}

// isWithinDir checks whether absPath is inside dir (both must be clean, absolute paths).
func isWithinDir(absPath, dir string) bool {
	rel, err := filepath.Rel(dir, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// extractAllowedDirs reads the allowed-dirs value from context and returns it.
func extractAllowedDirs(ctx context.Context) []string {
	dirs, _ := toolctx.AllowedDirs(ctx)
	return dirs
}

// ResolvePathWithWorkingDirectory resolves a path against the working directory from context.
// It handles:
// - Relative paths: resolved against working directory
// - Absolute paths within working directory or allowed directories: accepted
// - Absolute paths outside all permitted directories: rejected for security
//
// Additionally, the resolver refuses any path where the leaf or any
// existing ancestor up to the containing root is a symlink. A symlink
// in the parent chain would otherwise let a string-valid path escape
// to a real location outside the workspace at filesystem-traversal
// time. Tools that need to operate on symlinks should not exist in a
// workspace-restricted toolset.
func ResolvePathWithWorkingDirectory(ctx context.Context, inputPath string) (resolvedPath string, isValid bool) {
	// Extract working directory from context
	workingDir := "."
	if cwd, ok := toolctx.WorkingDir(ctx); ok && cwd != "" {
		workingDir = cwd
	}

	// Clean the input path
	inputPath = filepath.Clean(inputPath)
	workingDir = filepath.Clean(workingDir)

	// If path is relative, resolve against working directory and check bounds
	// (relative paths never resolve against allowed dirs — only cwd)
	if !filepath.IsAbs(inputPath) {
		resolvedPath = filepath.Join(workingDir, inputPath)

		absWorkingDir, err := filepath.Abs(workingDir)
		if err != nil {
			return "", false
		}

		absResolvedPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			return "", false
		}

		if !isWithinDir(absResolvedPath, absWorkingDir) {
			return "", false
		}

		if hasSymlinkComponent(absResolvedPath, absWorkingDir) {
			return "", false
		}

		return resolvedPath, true
	}

	// Path is absolute — check cwd first, then allowed dirs
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", false
	}

	if isWithinDir(inputPath, absWorkingDir) {
		if hasSymlinkComponent(inputPath, absWorkingDir) {
			return "", false
		}
		return inputPath, true
	}

	// Check allowed directories
	for _, dir := range extractAllowedDirs(ctx) {
		if isWithinDir(inputPath, dir) {
			if hasSymlinkComponent(inputPath, dir) {
				return "", false
			}
			return inputPath, true
		}
	}

	return "", false
}

// hasSymlinkComponent reports whether the leaf or any existing ancestor
// of absPath, up to (but excluding) absRoot, is a symlink. Non-existing
// path components are ignored — for write destinations the parent chain
// is the load-bearing thing, and any not-yet-created leaf can't be a
// symlink. absPath and absRoot must both be absolute and clean.
func hasSymlinkComponent(absPath, absRoot string) bool {
	cur := absPath
	for {
		if cur == absRoot {
			return false
		}
		if info, err := os.Lstat(cur); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return true
			}
		} else if !os.IsNotExist(err) {
			// Unexpected stat error — be conservative and reject.
			return true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached filesystem root before hitting the workspace
			// root (shouldn't happen given the prior containment
			// check, but be defensive).
			return false
		}
		cur = parent
	}
}

// ConvertToRelativePath converts an absolute path to relative from working directory
// Used for output formatting to ensure consistent relative paths
func ConvertToRelativePath(ctx context.Context, absolutePath string) string {
	// Extract working directory from context
	workingDir := "."
	if cwd, ok := toolctx.WorkingDir(ctx); ok && cwd != "" {
		workingDir = cwd
	}

	// Make working directory absolute for comparison
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return absolutePath
	}

	// Try to make path relative
	relPath, err := filepath.Rel(absWorkingDir, absolutePath)
	if err != nil {
		return absolutePath
	}

	// Don't return paths that go outside working directory
	if strings.HasPrefix(relPath, "..") {
		return absolutePath
	}

	return relPath
}
