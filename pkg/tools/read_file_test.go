package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileTool_Declaration(t *testing.T) {
	readFile := NewReadFileTool()
	decl := readFile.Declaration()
	
	if decl.Name != "readFile" {
		t.Errorf("Expected function name 'readFile', got %s", decl.Name)
	}
	
	if decl.Parameters == nil {
		t.Fatal("Expected parameters to be defined")
	}
	
	// Check required parameters
	if len(decl.Parameters.Required) != 1 || decl.Parameters.Required[0] != "file_path" {
		t.Errorf("Expected required parameter 'file_path', got %v", decl.Parameters.Required)
	}
	
	// Check file_path parameter exists
	if _, exists := decl.Parameters.Properties["file_path"]; !exists {
		t.Error("Expected file_path parameter to exist")
	}
	
	// Check line_numbers parameter exists
	if _, exists := decl.Parameters.Properties["line_numbers"]; !exists {
		t.Error("Expected line_numbers parameter to exist")
	}
}

func TestReadFileTool_Handler_ReadFile(t *testing.T) {
	// Setup test directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file.\nLine 3"
	
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	readFile := NewReadFileTool()
	handler := readFile.Handler()
	
	// Create context with working directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	params := map[string]any{
		"file_path": "test.txt",
	}
	
	result, err := handler(ctx, params)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	// Check result structure
	success, ok := result["success"].(bool)
	if !ok || !success {
		t.Errorf("Expected success to be true, got %v", result["success"])
	}
	
	content, ok := result["content"].(string)
	if !ok {
		t.Fatalf("Expected content to be string, got %T", result["content"])
	}
	
	if content != testContent {
		t.Errorf("Expected content %q, got %q", testContent, content)
	}
	
	// Should not have error field when successful
	if _, exists := result["error"]; exists {
		t.Error("Expected no error field when successful")
	}
}

func TestReadFileTool_Handler_ReadFileWithLineNumbers(t *testing.T) {
	// Setup test directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file.\nLine 3"
	
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	readFile := NewReadFileTool()
	handler := readFile.Handler()
	
	// Create context with working directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	params := map[string]any{
		"file_path":    "test.txt",
		"line_numbers": true,
	}
	
	result, err := handler(ctx, params)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	// Check result structure
	success, ok := result["success"].(bool)
	if !ok || !success {
		t.Errorf("Expected success to be true, got %v", result["success"])
	}
	
	content, ok := result["content"].(string)
	if !ok {
		t.Fatalf("Expected content to be string, got %T", result["content"])
	}
	
	// Content should have line numbers
	expectedContent := "     1\tHello, World!\n     2\tThis is a test file.\n     3\tLine 3"
	if content != expectedContent {
		t.Errorf("Expected content with line numbers:\n%q\nGot:\n%q", expectedContent, content)
	}
}

func TestReadFileTool_Handler_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	
	readFile := NewReadFileTool()
	handler := readFile.Handler()
	
	// Create context with working directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	params := map[string]any{
		"file_path": "nonexistent.txt",
	}
	
	result, err := handler(ctx, params)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	// Check result structure
	success, ok := result["success"].(bool)
	if !ok || success {
		t.Errorf("Expected success to be false, got %v", result["success"])
	}
	
	content, ok := result["content"].(string)
	if !ok {
		t.Fatalf("Expected content to be string, got %T", result["content"])
	}
	
	// Content should be empty for failed read
	if content != "" {
		t.Errorf("Expected empty content for failed read, got %q", content)
	}
	
	// Should have error message
	errorMsg, ok := result["error"].(string)
	if !ok || errorMsg == "" {
		t.Errorf("Expected error message, got %v", result["error"])
	}
	
	if !strings.Contains(errorMsg, "no such file or directory") && !strings.Contains(errorMsg, "cannot find the file") {
		t.Errorf("Expected file not found error, got %q", errorMsg)
	}
}

func TestReadFileTool_Handler_EmptyFile(t *testing.T) {
	// Setup test directory and empty file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "empty.txt")
	
	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	readFile := NewReadFileTool()
	handler := readFile.Handler()
	
	// Create context with working directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	params := map[string]any{
		"file_path": "empty.txt",
	}
	
	result, err := handler(ctx, params)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	// Check result structure
	success, ok := result["success"].(bool)
	if !ok || !success {
		t.Errorf("Expected success to be true, got %v", result["success"])
	}
	
	content, ok := result["content"].(string)
	if !ok {
		t.Fatalf("Expected content to be string, got %T", result["content"])
	}
	
	if content != "" {
		t.Errorf("Expected empty content, got %q", content)
	}
}

func TestReadFileTool_Handler_InvalidParameters(t *testing.T) {
	readFile := NewReadFileTool()
	handler := readFile.Handler()
	
	ctx := context.Background()
	
	tests := []struct {
		name   string
		params map[string]any
	}{
		{
			name:   "missing file_path",
			params: map[string]any{},
		},
		{
			name: "empty file_path",
			params: map[string]any{
				"file_path": "",
			},
		},
		{
			name: "non-string file_path",
			params: map[string]any{
				"file_path": 123,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler(ctx, tt.params)
			if err == nil {
				t.Error("Expected error for invalid parameters")
			}
		})
	}
}

func TestReadFileTool_Handler_PathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	
	readFile := NewReadFileTool()
	handler := readFile.Handler()
	
	// Create context with working directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	params := map[string]any{
		"file_path": "../../../etc/passwd",
	}
	
	result, err := handler(ctx, params)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	// Should fail due to path being outside working directory
	success, ok := result["success"].(bool)
	if !ok || success {
		t.Errorf("Expected success to be false for path traversal, got %v", result["success"])
	}
	
	errorMsg, ok := result["error"].(string)
	if !ok || !strings.Contains(errorMsg, "outside working directory") {
		t.Errorf("Expected 'outside working directory' error, got %q", errorMsg)
	}
}

func TestReadFileTool_Handler_AbsolutePathWithinWorkingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Test content"
	
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	readFile := NewReadFileTool()
	handler := readFile.Handler()
	
	// Create context with working directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	params := map[string]any{
		"file_path": testFile, // absolute path within working directory
	}
	
	result, err := handler(ctx, params)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	// Should succeed for absolute path within working directory
	success, ok := result["success"].(bool)
	if !ok || !success {
		t.Errorf("Expected success to be true for absolute path within working directory, got %v", result["success"])
	}
	
	content, ok := result["content"].(string)
	if !ok {
		t.Fatalf("Expected content to be string, got %T", result["content"])
	}
	
	if content != testContent {
		t.Errorf("Expected content %q, got %q", testContent, content)
	}
}

func TestReadFileTool_FormatOutput(t *testing.T) {
	readFile := &ReadFileTool{}
	
	tests := []struct {
		name     string
		result   map[string]interface{}
		expected string
	}{
		{
			name: "successful read",
			result: map[string]interface{}{
				"success": true,
				"content": "Hello, World!",
			},
			expected: "**File Content**\n```\nHello, World!\n```",
		},
		{
			name: "failed read with error",
			result: map[string]interface{}{
				"success": false,
				"content": "",
				"error":   "file not found",
			},
			expected: "**Failed to read file**: file not found",
		},
		{
			name: "failed read without error",
			result: map[string]interface{}{
				"success": false,
				"content": "",
			},
			expected: "**Failed to read file**",
		},
		{
			name: "empty file",
			result: map[string]interface{}{
				"success": true,
				"content": "",
			},
			expected: "**File is empty**",
		},
		{
			name: "long content gets truncated",
			result: map[string]interface{}{
				"success": true,
				"content": strings.Repeat("a", 1500),
			},
			expected: "**File Content**\n```\n" + strings.Repeat("a", 1000) + "\n... (truncated for display)\n```",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := readFile.FormatOutput(tt.result)
			if output != tt.expected {
				t.Errorf("Expected output:\n%q\nGot:\n%q", tt.expected, output)
			}
		})
	}
}

// TestReadFileTool_HandlesBothPathFormats tests that readFile handles both "file.txt" and "./file.txt" consistently
func TestReadFileTool_HandlesBothPathFormats(t *testing.T) {
	// Setup test directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "README.md")
	testContent := "# Test README\nThis is a test file."
	
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	
	readFile := NewReadFileTool()
	handler := readFile.Handler()
	
	// Create context with working directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	// Test different path formats that should all work the same
	pathFormats := []string{
		"README.md",      // simple relative path
		"./README.md",    // relative path with ./ prefix
	}
	
	for _, pathFormat := range pathFormats {
		t.Run(pathFormat, func(t *testing.T) {
			params := map[string]any{
				"file_path": pathFormat,
			}
			
			result, err := handler(ctx, params)
			require.NoError(t, err, "Handler should not return error for path: %s", pathFormat)
			
			// Check success
			success, ok := result["success"].(bool)
			require.True(t, ok, "success should be bool for path: %s", pathFormat)
			assert.True(t, success, "should succeed for path: %s", pathFormat)
			
			// Check content
			content, ok := result["content"].(string)
			require.True(t, ok, "content should be string for path: %s", pathFormat)
			assert.Equal(t, testContent, content, "content should match for path: %s", pathFormat)
			
			// Should not have error field when successful
			_, hasError := result["error"]
			assert.False(t, hasError, "should not have error field when successful for path: %s", pathFormat)
		})
	}
}