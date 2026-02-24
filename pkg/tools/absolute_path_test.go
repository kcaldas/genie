package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAbsolutePathHandling tests that tools handle absolute paths correctly
// when the LLM gets absolute paths from bash commands like pwd
func TestAbsolutePathHandling(t *testing.T) {
	// Create test directory structure
	testDir := t.TempDir()
	
	// Create subdirectories and files
	srcDir := filepath.Join(testDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	
	testFile := filepath.Join(srcDir, "main.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644))
	
	// Set up context with working directory
	ctx := context.WithValue(context.Background(), "cwd", testDir)
	
	// Initialize tools
	catTool := tools.NewReadFileTool(&events.NoOpPublisher{})
	writeTool := tools.NewWriteTool(nil, false)
	bashTool := tools.NewBashTool(nil, false)
	
	t.Run("LLM workflow with pwd and absolute paths", func(t *testing.T) {
		// Step 1: LLM runs pwd to get current directory
		result, err := bashTool.Handler()(ctx, map[string]any{
			"command": "pwd",
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		
		pwdOutput := result["results"].(string)
		// pwd should return the working directory
		assert.Contains(t, pwdOutput, testDir)
		
		// Step 2: LLM constructs absolute path and tries to read file
		absoluteFilePath := filepath.Join(testDir, "src", "main.go")
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path":        absoluteFilePath,
			"_display_message": "Testing reading file with absolute path within working directory",
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		assert.Contains(t, result["results"].(string), "package main")
		
		// Step 3: LLM tries to write to absolute path within working directory
		newAbsolutePath := filepath.Join(testDir, "src", "helper.go")
		result, err = writeTool.Handler()(ctx, map[string]any{
			"path":    newAbsolutePath,
			"content": "package main\n\nfunc helper() {}\n",
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		
		// Verify file was created
		content, err := os.ReadFile(newAbsolutePath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "func helper")
	})
	
	t.Run("Absolute paths outside working directory are rejected", func(t *testing.T) {
		// Try to read file outside working directory
		result, err := catTool.Handler()(ctx, map[string]any{
			"file_path": "/etc/passwd",
		})
		require.NoError(t, err)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"].(string), "outside working directory")
		
		// Try to write file outside working directory
		result, err = writeTool.Handler()(ctx, map[string]any{
			"path":    "/tmp/malicious.txt",
			"content": "malicious content",
		})
		require.NoError(t, err)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["results"], "outside working directory")
	})
	
	t.Run("Path traversal with absolute paths", func(t *testing.T) {
		// Create a file outside the working directory
		outsideDir := t.TempDir()
		outsideFile := filepath.Join(outsideDir, "secret.txt")
		require.NoError(t, os.WriteFile(outsideFile, []byte("secret content"), 0644))
		
		// Try to access it using path traversal with absolute path
		traversalPath := filepath.Join(testDir, "..", "..", "..", outsideFile)
		absoluteTraversalPath, _ := filepath.Abs(traversalPath)
		
		result, err := catTool.Handler()(ctx, map[string]any{
			"file_path": absoluteTraversalPath,
		})
		require.NoError(t, err)
		
		// Should be rejected since it's outside working directory
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"].(string), "outside working directory")
	})
	
	t.Run("Symlink handling with absolute paths", func(t *testing.T) {
		// Create a symlink within the working directory
		symlinkPath := filepath.Join(testDir, "link_to_main.go")
		targetPath := filepath.Join(testDir, "src", "main.go")
		
		err := os.Symlink(targetPath, symlinkPath)
		require.NoError(t, err)
		
		// Try to read using absolute path to symlink
		result, err := catTool.Handler()(ctx, map[string]any{
			"file_path":        symlinkPath,
			"_display_message": "Testing reading symlink with absolute path",
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		assert.Contains(t, result["results"].(string), "package main")
	})
	
	t.Run("Mixed relative and absolute paths work consistently", func(t *testing.T) {
		// Read same file with different path formats
		pathVariations := []string{
			"src/main.go",                           // relative
			"./src/main.go",                         // relative with ./
			filepath.Join(testDir, "src/main.go"),   // absolute within working dir
		}
		
		for _, path := range pathVariations {
			result, err := catTool.Handler()(ctx, map[string]any{
				"file_path":        path,
				"_display_message": "Testing consistent behavior across different path formats",
			})
			require.NoError(t, err, "Failed with path: %s", path)
			assert.True(t, result["success"].(bool), "Failed with path: %s", path)
			assert.Contains(t, result["results"].(string), "package main", "Wrong content for path: %s", path)
		}
	})
}

// TestAllowedDirectories tests that allowed directories expand the set of
// permitted absolute paths beyond the working directory.
func TestAllowedDirectories(t *testing.T) {
	cwd := t.TempDir()
	allowedDir1 := t.TempDir()
	allowedDir2 := t.TempDir()

	ctxBase := context.WithValue(context.Background(), "cwd", cwd)
	ctxWithAllowed := context.WithValue(ctxBase, "allowed_dirs", []string{allowedDir1, allowedDir2})

	t.Run("absolute path in first allowed dir is accepted", func(t *testing.T) {
		p := filepath.Join(allowedDir1, "file.txt")
		resolved, ok := tools.ResolvePathWithWorkingDirectory(ctxWithAllowed, p)
		assert.True(t, ok)
		assert.Equal(t, p, resolved)
	})

	t.Run("absolute path in second allowed dir is accepted", func(t *testing.T) {
		p := filepath.Join(allowedDir2, "sub", "deep.txt")
		resolved, ok := tools.ResolvePathWithWorkingDirectory(ctxWithAllowed, p)
		assert.True(t, ok)
		assert.Equal(t, p, resolved)
	})

	t.Run("absolute path outside cwd and allowed dirs is rejected", func(t *testing.T) {
		_, ok := tools.ResolvePathWithWorkingDirectory(ctxWithAllowed, "/etc/passwd")
		assert.False(t, ok)
	})

	t.Run("relative paths still resolve against cwd only", func(t *testing.T) {
		// Even with allowed dirs, relative paths must stay within cwd
		resolved, ok := tools.ResolvePathWithWorkingDirectory(ctxWithAllowed, "src/main.go")
		assert.True(t, ok)
		assert.Equal(t, filepath.Join(cwd, "src/main.go"), resolved)
	})

	t.Run("relative path traversal out of cwd is rejected even with allowed dirs", func(t *testing.T) {
		_, ok := tools.ResolvePathWithWorkingDirectory(ctxWithAllowed, "../../../etc/passwd")
		assert.False(t, ok)
	})

	t.Run("path traversal out of allowed dir is rejected", func(t *testing.T) {
		// Construct an absolute path that starts inside allowedDir1 but traverses out
		traversal := filepath.Join(allowedDir1, "..", "escape.txt")
		absTraversal, _ := filepath.Abs(traversal)
		_, ok := tools.ResolvePathWithWorkingDirectory(ctxWithAllowed, absTraversal)
		// absTraversal should be outside allowedDir1 after cleaning
		assert.False(t, ok)
	})

	t.Run("no allowed dirs behaves like before", func(t *testing.T) {
		// Without allowed_dirs, absolute path outside cwd is rejected
		p := filepath.Join(allowedDir1, "file.txt")
		_, ok := tools.ResolvePathWithWorkingDirectory(ctxBase, p)
		assert.False(t, ok)

		// Within cwd still works
		p2 := filepath.Join(cwd, "file.txt")
		resolved, ok := tools.ResolvePathWithWorkingDirectory(ctxBase, p2)
		assert.True(t, ok)
		assert.Equal(t, p2, resolved)
	})

	t.Run("ConvertToRelativePath returns absolute for allowed-dir paths", func(t *testing.T) {
		// Paths in allowed dirs are outside cwd, so ConvertToRelativePath returns them absolute
		p := filepath.Join(allowedDir1, "src", "lib.go")
		result := tools.ConvertToRelativePath(ctxWithAllowed, p)
		assert.Equal(t, p, result, "allowed-dir paths should stay absolute")
	})
}

// TestPathUtilityFunctions tests the path utility functions directly
func TestPathUtilityFunctions(t *testing.T) {
	testDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", testDir)
	
	t.Run("ResolvePathWithWorkingDirectory", func(t *testing.T) {
		testCases := []struct {
			name        string
			inputPath   string
			expectValid bool
			description string
		}{
			{"relative path", "src/main.go", true, "relative paths should be valid"},
			{"relative with dot", "./src/main.go", true, "relative paths with ./ should be valid"},
			{"absolute within working dir", filepath.Join(testDir, "src/main.go"), true, "absolute paths within working dir should be valid"},
			{"absolute outside working dir", "/etc/passwd", false, "absolute paths outside working dir should be invalid"},
			{"path traversal", "../../../etc/passwd", false, "path traversal outside working dir should be invalid"},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resolvedPath, isValid := tools.ResolvePathWithWorkingDirectory(ctx, tc.inputPath)
				assert.Equal(t, tc.expectValid, isValid, tc.description)
				
				if isValid {
					assert.NotEmpty(t, resolvedPath)
					// Valid paths should not contain .. components that escape working dir
					assert.NotContains(t, resolvedPath, "/../")
				}
			})
		}
	})
	
	t.Run("ConvertToRelativePath", func(t *testing.T) {
		// Test converting absolute paths to relative
		absolutePath := filepath.Join(testDir, "src", "main.go")
		relativePath := tools.ConvertToRelativePath(ctx, absolutePath)
		
		expected := filepath.Join("src", "main.go")
		assert.Equal(t, expected, relativePath)
		
		// Test with path outside working directory (should return original)
		outsidePath := "/etc/passwd"
		result := tools.ConvertToRelativePath(ctx, outsidePath)
		assert.Equal(t, outsidePath, result)
	})
}