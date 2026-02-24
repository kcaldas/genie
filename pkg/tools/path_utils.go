package tools

import (
	"context"
	"path/filepath"
	"strings"
)

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