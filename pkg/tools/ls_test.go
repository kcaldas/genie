package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLsTool_Declaration(t *testing.T) {
	tool := NewLsTool()
	declaration := tool.Declaration()
	
	assert.Equal(t, "listFiles", declaration.Name)
	assert.Contains(t, declaration.Description, "recursive")
	
	// Check new parameters exist
	params := declaration.Parameters.Properties
	assert.Contains(t, params, "max_depth")
	assert.Contains(t, params, "files_only")
	assert.Contains(t, params, "dirs_only")
	assert.Contains(t, params, "max_results")
	
	// Check max_depth constraints
	maxDepthSchema := params["max_depth"]
	assert.Equal(t, float64(1), maxDepthSchema.Minimum)
	assert.Equal(t, float64(10), maxDepthSchema.Maximum)
}

func TestLsTool_ParseListParams(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]any
		expected listConfig
	}{
		{
			name:   "default values",
			params: map[string]any{},
			expected: listConfig{
				path:       ".",
				maxDepth:   DefaultListDepth,
				showHidden: false,
				longFormat: false,
				filesOnly:  false,
				dirsOnly:   false,
				maxResults: 200,
			},
		},
		{
			name: "single directory mode",
			params: map[string]any{
				"max_depth": float64(1),
			},
			expected: listConfig{
				path:       ".",
				maxDepth:   1,
				showHidden: false,
				longFormat: false,
				filesOnly:  false,
				dirsOnly:   false,
				maxResults: 0, // unlimited for single directory
			},
		},
		{
			name: "custom recursive settings",
			params: map[string]any{
				"path":         "pkg",
				"max_depth":    float64(5),
				"show_hidden":  true,
				"files_only":   true,
				"max_results":  float64(100),
			},
			expected: listConfig{
				path:       "pkg",
				maxDepth:   5,
				showHidden: true,
				longFormat: false,
				filesOnly:  true,
				dirsOnly:   false,
				maxResults: 100,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseListParams(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLsTool_SingleDirectoryMode(t *testing.T) {
	// Create a temporary directory with some files
	tempDir := t.TempDir()
	
	// Create test files
	testFiles := []string{"file1.txt", "file2.go", ".hidden"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("test"), 0644)
		require.NoError(t, err)
	}
	
	tool := NewLsTool()
	handler := tool.Handler()
	
	tests := []struct {
		name     string
		params   map[string]any
		validate func(t *testing.T, result map[string]any)
	}{
		{
			name: "basic single directory listing",
			params: map[string]any{
				"path":      tempDir,
				"max_depth": float64(1),
			},
			validate: func(t *testing.T, result map[string]any) {
				assert.True(t, result["success"].(bool))
				files := result["files"].(string)
				assert.Contains(t, files, "file1.txt")
				assert.Contains(t, files, "file2.go")
				assert.NotContains(t, files, ".hidden") // hidden files not shown by default
			},
		},
		{
			name: "show hidden files",
			params: map[string]any{
				"path":        tempDir,
				"max_depth":   float64(1),
				"show_hidden": true,
			},
			validate: func(t *testing.T, result map[string]any) {
				assert.True(t, result["success"].(bool))
				files := result["files"].(string)
				assert.Contains(t, files, ".hidden")
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			result, err := handler(ctx, tt.params)
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
}

func TestLsTool_RecursiveMode(t *testing.T) {
	// Create a nested directory structure
	tempDir := t.TempDir()
	
	// Create nested structure
	structure := map[string]string{
		"file1.txt":           "root file",
		"dir1/file2.txt":      "dir1 file",
		"dir1/subdir/file3.go": "nested file",
		"dir2/file4.js":       "dir2 file",
		".hidden/secret.txt":  "hidden dir file",
	}
	
	for path, content := range structure {
		fullPath := filepath.Join(tempDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}
	
	tool := NewLsTool()
	handler := tool.Handler()
	
	tests := []struct {
		name     string
		params   map[string]any
		validate func(t *testing.T, result map[string]any)
	}{
		{
			name: "default recursive listing (depth 3)",
			params: map[string]any{
				"path": tempDir,
			},
			validate: func(t *testing.T, result map[string]any) {
				assert.True(t, result["success"].(bool))
				files := result["files"].(string)
				lines := strings.Split(files, "\n")
				
				// Should contain root directory
				assert.Contains(t, lines, "./")
				
				// Should contain nested files (depth 3 default)
				found := false
				for _, line := range lines {
					if strings.Contains(line, "file3.go") {
						found = true
						break
					}
				}
				assert.True(t, found, "Should find nested file at depth 3")
			},
		},
		{
			name: "limited depth",
			params: map[string]any{
				"path":      tempDir,
				"max_depth": float64(2),
			},
			validate: func(t *testing.T, result map[string]any) {
				assert.True(t, result["success"].(bool))
				files := result["files"].(string)
				
				// Should not contain deeply nested files
				assert.NotContains(t, files, "file3.go")
				// Should contain level 2 files
				assert.Contains(t, files, "dir1")
			},
		},
		{
			name: "files only",
			params: map[string]any{
				"path":       tempDir,
				"files_only": true,
			},
			validate: func(t *testing.T, result map[string]any) {
				assert.True(t, result["success"].(bool))
				files := result["files"].(string)
				lines := strings.Split(files, "\n")
				
				// Should not contain directories
				for _, line := range lines {
					if line != "" {
						assert.True(t, strings.Contains(line, ".txt") || 
									strings.Contains(line, ".go") || 
									strings.Contains(line, ".js"),
							"Should only contain files, got: %s", line)
					}
				}
			},
		},
		{
			name: "directories only",
			params: map[string]any{
				"path":      tempDir,
				"dirs_only": true,
			},
			validate: func(t *testing.T, result map[string]any) {
				assert.True(t, result["success"].(bool))
				files := result["files"].(string)
				lines := strings.Split(files, "\n")
				
				// Should contain directories
				found := false
				for _, line := range lines {
					if strings.Contains(line, "dir1") {
						found = true
						break
					}
				}
				assert.True(t, found, "Should contain directories")
			},
		},
		{
			name: "max results limit",
			params: map[string]any{
				"path":        tempDir,
				"max_results": float64(3),
			},
			validate: func(t *testing.T, result map[string]any) {
				assert.True(t, result["success"].(bool))
				files := result["files"].(string)
				lines := strings.Split(strings.TrimSpace(files), "\n")
				
				// Should be limited to max results
				assert.LessOrEqual(t, len(lines), 3)
				assert.Equal(t, 3, result["count"].(int))
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			result, err := handler(ctx, tt.params)
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
}

func TestLsTool_GitignoreSupport(t *testing.T) {
	// Create a temporary directory with .gitignore
	tempDir := t.TempDir()
	
	// Create .gitignore file
	gitignoreContent := `node_modules/
*.log
dist
.env
`
	err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644)
	require.NoError(t, err)
	
	// Create files that should be ignored
	filesToCreate := []string{
		"src/index.js",
		"node_modules/package/index.js",
		"app.log",
		"dist/bundle.js",
		".env",
		"README.md",
	}
	
	for _, file := range filesToCreate {
		fullPath := filepath.Join(tempDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte("content"), 0644)
		require.NoError(t, err)
	}
	
	tool := NewLsTool()
	handler := tool.Handler()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result, err := handler(ctx, map[string]any{"path": tempDir})
	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
	
	files := result["files"].(string)
	
	// Should contain allowed files
	assert.Contains(t, files, "README.md")
	assert.Contains(t, files, "index.js") // in src/
	
	// Should not contain ignored files
	assert.NotContains(t, files, "node_modules")
	assert.NotContains(t, files, "app.log")
	assert.NotContains(t, files, "dist")
	assert.NotContains(t, files, ".env")
}

func TestLsTool_ErrorHandling(t *testing.T) {
	tool := NewLsTool()
	handler := tool.Handler()
	
	tests := []struct {
		name     string
		params   map[string]any
		validate func(t *testing.T, result map[string]any)
	}{
		{
			name: "nonexistent directory",
			params: map[string]any{
				"path":      "/nonexistent/directory",
				"max_depth": float64(1),
			},
			validate: func(t *testing.T, result map[string]any) {
				assert.False(t, result["success"].(bool))
				assert.Contains(t, result["error"].(string), "failed")
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			result, err := handler(ctx, tt.params)
			require.NoError(t, err) // Handler shouldn't return Go errors
			tt.validate(t, result)
		})
	}
}

func TestLsTool_ContextCancellation(t *testing.T) {
	// Create a large directory structure for testing cancellation
	tempDir := t.TempDir()
	
	// Create many nested directories to make walk take some time
	for i := 0; i < 100; i++ {
		dir := filepath.Join(tempDir, "dir", "subdir", "level3", "level4")
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		
		file := filepath.Join(dir, "file.txt")
		err = os.WriteFile(file, []byte("content"), 0644)
		require.NoError(t, err)
	}
	
	tool := NewLsTool()
	handler := tool.Handler()
	
	// Create a context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	
	result, err := handler(ctx, map[string]any{
		"path":      tempDir,
		"max_depth": float64(10),
	})
	
	// Should handle cancellation gracefully
	require.NoError(t, err)
	// May succeed if it finishes quickly, or fail due to cancellation
	if !result["success"].(bool) {
		// If it failed, should be due to context cancellation
		assert.Contains(t, result["error"].(string), "context")
	}
}

// TestLsTool_RelativePathOutput tests that listFiles always returns relative paths
// regardless of the working directory, so LLM sees clean paths
func TestLsTool_RelativePathOutput(t *testing.T) {
	// Create a test directory structure
	testDir := t.TempDir()
	
	// Create subdirectories and files
	srcDir := filepath.Join(testDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	
	// Create files
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "README.md"), []byte("readme"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "utils.go"), []byte("package main"), 0644))
	
	tool := NewLsTool()
	handler := tool.Handler()
	
	t.Run("working directory with long path should show relative paths", func(t *testing.T) {
		// Simulate starting genie from a deeply nested directory
		ctx := context.WithValue(context.Background(), "cwd", testDir)
		
		// List current directory
		result, err := handler(ctx, map[string]any{
			"path": ".",
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		
		files := result["files"].(string)
		
		// Files should be relative, not absolute paths
		assert.Contains(t, files, "README.md", "Should show relative path for file in root")
		assert.Contains(t, files, "src/", "Should show relative path for subdirectory")
		
		// Should NOT contain the full working directory path
		assert.NotContains(t, files, testDir, "Output should not contain absolute working directory path")
		
		// Should not contain any absolute paths at all
		lines := strings.Split(strings.TrimSpace(files), "\n")
		for _, line := range lines {
			// Extract just the path part (first field)
			fields := strings.Fields(line)
			if len(fields) > 0 {
				path := fields[len(fields)-1] // Last field is usually the path
				assert.False(t, filepath.IsAbs(path), "Path should be relative: %s", path)
				assert.False(t, strings.Contains(path, testDir), "Path should not contain working directory: %s", path)
			}
		}
	})
	
	t.Run("recursive listing should show relative paths", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "cwd", testDir)
		
		// Recursive list
		result, err := handler(ctx, map[string]any{
			"path":      ".",
			"max_depth": 3,
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		
		files := result["files"].(string)
		
		// Should show relative paths for nested files
		assert.Contains(t, files, "src/main.go", "Should show relative path for nested file")
		assert.Contains(t, files, "src/utils.go", "Should show relative path for nested file")
		
		// Should NOT contain absolute paths
		assert.NotContains(t, files, testDir, "Output should not contain absolute working directory path")
		
		// Verify all paths are relative
		lines := strings.Split(strings.TrimSpace(files), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				assert.False(t, filepath.IsAbs(line), "All paths should be relative: %s", line)
			}
		}
	})
	
	t.Run("different working directory formats should produce same relative output", func(t *testing.T) {
		// Test with different working directory representations
		workingDirs := []string{
			testDir,                    // absolute path
			filepath.Clean(testDir),    // cleaned absolute path
		}
		
		var outputs []string
		for _, wd := range workingDirs {
			ctx := context.WithValue(context.Background(), "cwd", wd)
			
			result, err := handler(ctx, map[string]any{
				"path": ".",
			})
			require.NoError(t, err)
			assert.True(t, result["success"].(bool))
			
			outputs = append(outputs, result["files"].(string))
		}
		
		// All outputs should be identical (relative paths)
		for i := 1; i < len(outputs); i++ {
			assert.Equal(t, outputs[0], outputs[i], "Output should be identical regardless of working directory format")
		}
	})
}