package tools

import (
	"context"
	"path/filepath"
	"strings"
)

// ResolvePathWithWorkingDirectory resolves a path against the working directory from context.
// It handles:
// - Relative paths: resolved against working directory
// - Absolute paths within working directory: converted to relative
// - Absolute paths outside working directory: rejected for security
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
	if !filepath.IsAbs(inputPath) {
		resolvedPath = filepath.Join(workingDir, inputPath)
		
		// Check if the resolved path escapes the working directory
		absWorkingDir, err := filepath.Abs(workingDir)
		if err != nil {
			return "", false
		}
		
		absResolvedPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			return "", false
		}
		
		relPath, err := filepath.Rel(absWorkingDir, absResolvedPath)
		if err != nil {
			return "", false
		}
		
		// If relative path starts with "..", it escapes working directory
		if strings.HasPrefix(relPath, "..") {
			return "", false
		}
		
		return resolvedPath, true
	}
	
	// Path is absolute - check if it's within working directory
	// First, ensure working directory is absolute for comparison
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", false
	}
	
	// Check if the absolute path is within the working directory
	relPath, err := filepath.Rel(absWorkingDir, inputPath)
	if err != nil {
		return "", false
	}
	
	// If relative path starts with "..", it's outside working directory
	if strings.HasPrefix(relPath, "..") {
		return "", false
	}
	
	// Path is within working directory, return the absolute path
	// (tools will handle it correctly since it's within bounds)
	return inputPath, true
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