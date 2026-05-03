package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// WorkingDirectoryFromContext returns the workspace root the agent is bound
// to. Defaults to "." when the context carries no `cwd` value. Exported so
// tools can include it in error messages — the model needs to know what
// the workspace is to recover (e.g. retry with a path inside it).
func WorkingDirectoryFromContext(ctx context.Context) string {
	if v := ctx.Value("cwd"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "."
}

// AllowedDirsFromContext returns the additional read-allowed directories
// the agent has been granted (besides the workspace root). Empty when none
// are configured.
func AllowedDirsFromContext(ctx context.Context) []string {
	return extractAllowedDirs(ctx)
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

// extractAllowedDirs reads the "allowed_dirs" value from context and returns it.
func extractAllowedDirs(ctx context.Context) []string {
	if v := ctx.Value("allowed_dirs"); v != nil {
		if dirs, ok := v.([]string); ok {
			return dirs
		}
	}
	return nil
}

// ResolvePathWithWorkingDirectory resolves a path against the working directory from context.
// It handles:
// - Relative paths: resolved against working directory
// - Absolute paths within working directory or allowed directories: accepted
// - Absolute paths outside all permitted directories: rejected for security
func ResolvePathWithWorkingDirectory(ctx context.Context, inputPath string) (resolvedPath string, isValid bool) {
	// Extract working directory from context
	workingDir := "."
	if cwd := ctx.Value("cwd"); cwd != nil {
		if cwdStr, ok := cwd.(string); ok && cwdStr != "" {
			workingDir = cwdStr
		}
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

		return resolvedPath, true
	}

	// Path is absolute — check cwd first, then allowed dirs
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", false
	}

	if isWithinDir(inputPath, absWorkingDir) {
		return inputPath, true
	}

	// Check allowed directories
	for _, dir := range extractAllowedDirs(ctx) {
		if isWithinDir(inputPath, dir) {
			return inputPath, true
		}
	}

	return "", false
}

// ConvertToRelativePath converts an absolute path to relative from working directory
// Used for output formatting to ensure consistent relative paths
func ConvertToRelativePath(ctx context.Context, absolutePath string) string {
	// Extract working directory from context
	workingDir := "."
	if cwd := ctx.Value("cwd"); cwd != nil {
		if cwdStr, ok := cwd.(string); ok && cwdStr != "" {
			workingDir = cwdStr
		}
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