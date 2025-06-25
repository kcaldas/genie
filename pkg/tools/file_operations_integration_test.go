package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileOperationsToolsIntegration simulates common LLM workflows using file operation tools
// This ensures path handling is consistent across all tools when used together
func TestFileOperationsToolsIntegration(t *testing.T) {
	// Create a realistic project structure
	projectDir := t.TempDir()
	
	// Create directory structure
	dirs := []string{
		"src",
		"src/models",
		"src/controllers",
		"tests",
		"docs",
		".git",
	}
	
	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(projectDir, dir), 0755))
	}
	
	// Create test files
	files := map[string]string{
		"README.md":                    "# Test Project\n\nThis is a test project for file operations.",
		"package.json":                 `{"name": "test-project", "version": "1.0.0"}`,
		".gitignore":                   "node_modules/\n*.log\n.env",
		"src/index.js":                 "const app = require('./app');\n\napp.listen(3000);",
		"src/app.js":                   "const express = require('express');\n\nconst app = express();\n\nmodule.exports = app;",
		"src/models/user.js":           "class User {\n  constructor(name) {\n    this.name = name;\n  }\n}\n\nmodule.exports = User;",
		"src/controllers/userController.js": "const User = require('../models/user');\n\nfunction getUser(id) {\n  // TODO: implement\n}\n\nmodule.exports = { getUser };",
		"tests/user.test.js":           "const User = require('../src/models/user');\n\ntest('creates user', () => {\n  const user = new User('John');\n  expect(user.name).toBe('John');\n});",
		"docs/API.md":                  "# API Documentation\n\n## GET /users/:id\n\nReturns user by ID.",
	}
	
	for path, content := range files {
		fullPath := filepath.Join(projectDir, path)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}
	
	// Set up context with working directory
	ctx := context.WithValue(context.Background(), "cwd", projectDir)
	
	// Initialize all tools
	lsTool := tools.NewLsTool()
	catTool := tools.NewReadFileTool()
	writeTool := tools.NewWriteTool(nil, nil, false)
	findTool := tools.NewFindTool()
	grepTool := tools.NewGrepTool()
	
	t.Run("Workflow 1: Explore project structure and read files", func(t *testing.T) {
		// Step 1: LLM lists files to understand project structure
		result, err := lsTool.Handler()(ctx, map[string]any{
			"path": ".",
			"max_depth": 3,
		})
		require.NoError(t, err)
		
		filesList := result["results"].(string)
		
		// Verify we see the expected structure with relative paths
		assert.Contains(t, filesList, "./README.md")
		assert.Contains(t, filesList, "./src/index.js")
		assert.Contains(t, filesList, "./src/models/user.js")
		
		// Step 2: LLM reads README using path from ls output
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path": "./README.md", // Using path as shown in ls output
		})
		require.NoError(t, err)
		assert.Contains(t, result["results"].(string), "# Test Project")
		
		// Step 3: LLM reads a nested file using path from ls
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path": "./src/models/user.js", // Using path as shown in ls output
		})
		require.NoError(t, err)
		assert.Contains(t, result["results"].(string), "class User")
		
		// Step 4: LLM reads the same file without ./ prefix (common variation)
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path": "src/models/user.js",
		})
		require.NoError(t, err)
		assert.Contains(t, result["results"].(string), "class User")
	})
	
	t.Run("Workflow 2: Search for files and modify them", func(t *testing.T) {
		// Step 1: LLM searches for test files
		result, err := findTool.Handler()(ctx, map[string]any{
			"pattern": "*.test.js",
			"path": ".",
		})
		require.NoError(t, err)
		
		foundFiles := result["results"].(string)
		assert.Contains(t, foundFiles, "tests/user.test.js")
		
		// Step 2: LLM reads the test file using path from find output
		testPath := "tests/user.test.js" // Path as it appears in find output
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path": testPath,
		})
		require.NoError(t, err)
		assert.Contains(t, result["results"].(string), "test('creates user'")
		
		// Step 3: LLM modifies the test file
		newContent := strings.Replace(result["results"].(string), "John", "Jane", -1)
		result, err = writeTool.Handler()(ctx, map[string]any{
			"path": testPath, // Using same path format
			"content": newContent,
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		
		// Step 4: Verify the change
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path": testPath,
		})
		require.NoError(t, err)
		assert.Contains(t, result["results"].(string), "Jane")
		assert.NotContains(t, result["results"].(string), "John")
	})
	
	t.Run("Workflow 3: Search content and create related files", func(t *testing.T) {
		// Step 1: LLM searches for TODO comments
		result, err := grepTool.Handler()(ctx, map[string]any{
			"pattern": "TODO",
			"path": ".",
		})
		require.NoError(t, err)
		
		matches := result["results"].(string)
		assert.Contains(t, matches, "src/controllers/userController.js")
		
		// Step 2: LLM reads the file with TODO
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path": "src/controllers/userController.js",
		})
		require.NoError(t, err)
		
		// Step 3: LLM creates a new file in the same directory
		result, err = writeTool.Handler()(ctx, map[string]any{
			"path": "src/controllers/productController.js",
			"content": "const Product = require('../models/product');\n\nfunction getProduct(id) {\n  // Implementation\n}\n\nmodule.exports = { getProduct };",
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		
		// Step 4: Verify new file exists and can be read
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path": "./src/controllers/productController.js", // With ./ prefix
		})
		require.NoError(t, err)
		assert.Contains(t, result["results"].(string), "getProduct")
	})
	
	t.Run("Workflow 4: Navigate directories and work with relative paths", func(t *testing.T) {
		// Step 1: List only the src directory
		result, err := lsTool.Handler()(ctx, map[string]any{
			"path": "src",
			"max_depth": 2,
		})
		require.NoError(t, err)
		
		filesList := result["results"].(string)
		// Paths should be relative to project root, not src
		assert.Contains(t, filesList, "./src/index.js")
		assert.Contains(t, filesList, "./src/models/user.js")
		
		// Step 2: Find all .js files in src
		result, err = findTool.Handler()(ctx, map[string]any{
			"pattern": "*.js",
			"path": "src",
		})
		require.NoError(t, err)
		
		foundFiles := result["results"].(string)
		// All paths should include src/ prefix
		assert.Contains(t, foundFiles, "src/index.js")
		assert.Contains(t, foundFiles, "src/app.js")
		assert.Contains(t, foundFiles, "src/models/user.js")
		
		// Step 3: Create a new model file
		result, err = writeTool.Handler()(ctx, map[string]any{
			"path": "src/models/product.js",
			"content": "class Product {\n  constructor(name, price) {\n    this.name = name;\n    this.price = price;\n  }\n}\n\nmodule.exports = Product;",
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
	})
	
	t.Run("Workflow 5: Complex path variations LLM might use", func(t *testing.T) {
		// Test that all these path variations work correctly
		pathVariations := []struct {
			desc     string
			path     string
			expected string
		}{
			{"simple relative", "README.md", "# Test Project"},
			{"with dot slash", "./README.md", "# Test Project"},
			{"nested no prefix", "src/index.js", "app.listen"},
			{"nested with dot slash", "./src/index.js", "app.listen"},
			{"deep nested", "src/models/user.js", "class User"},
			{"deep with dot slash", "./src/models/user.js", "class User"},
		}
		
		for _, tc := range pathVariations {
			result, err := catTool.Handler()(ctx, map[string]any{
				"file_path": tc.path,
			})
			require.NoError(t, err, "Failed for %s: %s", tc.desc, tc.path)
			assert.Contains(t, result["results"].(string), tc.expected, 
				"Wrong content for %s: %s", tc.desc, tc.path)
		}
	})
	
	t.Run("Workflow 6: Creating nested directories", func(t *testing.T) {
		// LLM tries to create a file in a new nested directory
		result, err := writeTool.Handler()(ctx, map[string]any{
			"path": "src/utils/helpers.js",
			"content": "function formatDate(date) {\n  return date.toISOString();\n}\n\nmodule.exports = { formatDate };",
		})
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		
		// Verify the directory was created and file exists
		assert.DirExists(t, filepath.Join(projectDir, "src/utils"))
		
		// Read the file back
		result, err = catTool.Handler()(ctx, map[string]any{
			"file_path": "src/utils/helpers.js",
		})
		require.NoError(t, err)
		assert.Contains(t, result["results"].(string), "formatDate")
	})
}

// TestFileOperationsErrorHandling tests error cases in file operations
func TestFileOperationsErrorHandling(t *testing.T) {
	projectDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", projectDir)
	
	catTool := tools.NewReadFileTool()
	writeTool := tools.NewWriteTool(nil, nil, false)
	
	t.Run("Reading non-existent file", func(t *testing.T) {
		result, err := catTool.Handler()(ctx, map[string]any{
			"file_path": "does-not-exist.txt",
		})
		require.NoError(t, err) // Handler returns error in result
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"].(string), "failed to read file")
	})
	
	t.Run("Writing with absolute path is rejected", func(t *testing.T) {
		result, err := writeTool.Handler()(ctx, map[string]any{
			"path": "/etc/passwd",
			"content": "malicious content",
		})
		require.NoError(t, err)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["results"], "outside working directory")
	})
	
	t.Run("Path traversal attempts are handled", func(t *testing.T) {
		// Create a file
		testFile := filepath.Join(projectDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))
		
		// Try to read with path traversal
		result, err := catTool.Handler()(ctx, map[string]any{
			"file_path": "../../../../../etc/passwd",
		})
		require.NoError(t, err)
		// This should either fail or read a file within the working directory
		// depending on path resolution, but should NOT read /etc/passwd
		if success, ok := result["success"].(bool); ok && success {
			content := result["results"].(string)
			assert.NotContains(t, content, "root:") // Should not contain passwd file content
		}
	})
}