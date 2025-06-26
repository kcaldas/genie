package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// LsTool lists files and directories
type LsTool struct {
	publisher events.Publisher
}

// listConfig holds configuration for listing operation
type listConfig struct {
	path       string
	maxDepth   int
	showHidden bool
	longFormat bool
	filesOnly  bool
	dirsOnly   bool
	maxResults int
}

// DefaultListDepth is the default recursion depth
const DefaultListDepth = 3

// loadGitignorePatterns loads patterns from .gitignore file if it exists
func loadGitignorePatterns(rootPath string) []string {
	gitignorePath := filepath.Join(rootPath, ".gitignore")

	file, err := os.Open(gitignorePath)
	if err != nil {
		// No .gitignore file, return empty patterns
		return []string{}
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns
}

// shouldIgnore checks if a path should be ignored based on gitignore patterns
func shouldIgnore(path string, patterns []string) bool {
	baseName := filepath.Base(path)

	for _, pattern := range patterns {
		// Simple pattern matching - handle basic cases
		if matched, _ := filepath.Match(pattern, baseName); matched {
			return true
		}
		// Handle directory patterns (ending with /)
		if strings.HasSuffix(pattern, "/") {
			dirPattern := strings.TrimSuffix(pattern, "/")
			if matched, _ := filepath.Match(dirPattern, baseName); matched {
				return true
			}
		}
	}
	return false
}

// parseListParams extracts configuration from parameters
func parseListParams(params map[string]any) listConfig {
	config := listConfig{
		path:       ".",
		maxDepth:   DefaultListDepth,
		showHidden: false,
		longFormat: false,
		filesOnly:  false,
		dirsOnly:   false,
		maxResults: 200,
	}

	if pathParam, exists := params["path"]; exists {
		if pathStr, ok := pathParam.(string); ok && pathStr != "" {
			config.path = pathStr
		}
	}

	if depthParam, exists := params["max_depth"]; exists {
		if depthInt, ok := depthParam.(float64); ok {
			config.maxDepth = int(depthInt)
		}
	}

	if hiddenParam, exists := params["show_hidden"]; exists {
		if hiddenBool, ok := hiddenParam.(bool); ok {
			config.showHidden = hiddenBool
		}
	}

	if longParam, exists := params["long_format"]; exists {
		if longBool, ok := longParam.(bool); ok {
			config.longFormat = longBool
		}
	}

	if filesParam, exists := params["files_only"]; exists {
		if filesBool, ok := filesParam.(bool); ok {
			config.filesOnly = filesBool
		}
	}

	if dirsParam, exists := params["dirs_only"]; exists {
		if dirsBool, ok := dirsParam.(bool); ok {
			config.dirsOnly = dirsBool
		}
	}

	if maxParam, exists := params["max_results"]; exists {
		if maxInt, ok := maxParam.(float64); ok {
			config.maxResults = int(maxInt)
		}
	}

	// Adjust defaults for single directory mode
	if config.maxDepth == 1 {
		config.maxResults = 0 // unlimited for single directory
	}

	return config
}

// NewLsTool creates a new ls tool
func NewLsTool(publisher events.Publisher) Tool {
	return &LsTool{
		publisher: publisher,
	}
}

// Declaration returns the function declaration for the ls tool
func (l *LsTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "listFiles",
		Description: "List files and directories recursively. Default depth of 3 levels provides good project overview. Use max_depth=1 for single directory listing like 'ls'.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for listing files and directories",
			Properties: map[string]*ai.Schema{
				"path": {
					Type:        ai.TypeString,
					Description: "Path to list (default: '.'). Examples: '.', '/path/to/dir', 'src/'",
					MaxLength:   500,
				},
				"max_depth": {
					Type:        ai.TypeInteger,
					Description: "Maximum depth to recurse (default: 3, use 1 for single directory, max: 10)",
					Minimum:     1,
					Maximum:     10,
				},
				"show_hidden": {
					Type:        ai.TypeBoolean,
					Description: "Show hidden files (files starting with .)",
				},
				"long_format": {
					Type:        ai.TypeBoolean,
					Description: "Show detailed information (permissions, size, date) - only works with max_depth=1",
				},
				"files_only": {
					Type:        ai.TypeBoolean,
					Description: "Show only files, exclude directories (for recursive listing)",
				},
				"dirs_only": {
					Type:        ai.TypeBoolean,
					Description: "Show only directories, exclude files (for recursive listing)",
				},
				"max_results": {
					Type:        ai.TypeInteger,
					Description: "Maximum number of entries to return (default: 200 for recursive, unlimited for single directory)",
					Minimum:     10,
					Maximum:     1000,
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "Required message explaining why you are listing these files. Tell the user what you're looking for or what you plan to analyze.",
					MinLength:   5,
					MaxLength:   200,
				},
			},
			Required: []string{"_display_message"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "List of files and directories",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the listing was successful",
				},
				"results": {
					Type:        ai.TypeString,
					Description: "The file listing output",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if listing failed",
				},
			},
			Required: []string{"success", "results"},
		},
	}
}

// Handler returns the function handler for the ls tool
func (l *LsTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		config := parseListParams(params)

		// Check for required display message and publish event
		if l.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				l.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "listFiles",
					Message:  msg,
				})
			} else {
				return nil, fmt.Errorf("_display_message parameter is required")
			}
		}

		// Extract working directory from context
		workingDir := "."
		if cwd := ctx.Value("cwd"); cwd != nil {
			if cwdStr, ok := cwd.(string); ok && cwdStr != "" {
				workingDir = cwdStr
			}
		}

		// Resolve relative paths against working directory
		if !filepath.IsAbs(config.path) {
			config.path = filepath.Join(workingDir, config.path)
		}

		if config.maxDepth == 1 {
			// Single directory mode - use existing ls command logic
			return l.handleSingleDirectory(ctx, config)
		} else {
			// Recursive mode - use filepath.Walk
			return l.handleRecursiveDirectory(ctx, config)
		}
	}
}

// handleSingleDirectory uses ls command for single directory listing
func (l *LsTool) handleSingleDirectory(ctx context.Context, config listConfig) (map[string]any, error) {
	// Build ls command
	args := []string{}

	// Check for long format
	if config.longFormat {
		args = append(args, "-l")
	}

	// Check for hidden files
	if config.showHidden {
		args = append(args, "-a")
	}

	// Add path
	args = append(args, config.path)

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Execute ls command
	cmd := exec.CommandContext(execCtx, "ls", args...)
	cmd.Env = os.Environ()

	// Extract working directory from context for exec
	if cwd := ctx.Value("cwd"); cwd != nil {
		if cwdStr, ok := cwd.(string); ok && cwdStr != "" {
			cmd.Dir = cwdStr
		}
	}

	output, err := cmd.CombinedOutput()

	// Check for timeout
	if execCtx.Err() == context.DeadlineExceeded {
		return map[string]any{
			"success": false,
			"results": string(output),
			"error":   "command timed out",
		}, nil
	}

	// Check for other errors
	if err != nil {
		return map[string]any{
			"success": false,
			"results": string(output),
			"error":   fmt.Sprintf("ls failed: %v", err),
		}, nil
	}

	return map[string]any{
		"success": true,
		"results": string(output),
	}, nil
}

// handleRecursiveDirectory uses filepath.Walk for recursive listing
func (l *LsTool) handleRecursiveDirectory(ctx context.Context, config listConfig) (map[string]any, error) {
	var paths []string
	count := 0

	// Load gitignore patterns from root path
	gitignorePatterns := loadGitignorePatterns(config.path)

	// Get absolute path for depth calculation
	absRoot, err := filepath.Abs(config.path)
	if err != nil {
		return map[string]any{
			"success": false,
			"results": "",
			"error":   fmt.Sprintf("failed to get absolute path: %v", err),
		}, nil
	}

	err = filepath.Walk(config.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip errors, continue walking
			return nil
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate depth
		absPath, _ := filepath.Abs(path)
		relPath, _ := filepath.Rel(absRoot, absPath)
		depth := strings.Count(relPath, string(filepath.Separator))
		if relPath != "." {
			depth++ // Add 1 for the file/dir itself
		}

		// Skip if too deep
		if depth > config.maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files if not requested (but don't skip the root directory)
		if !config.showHidden && strings.HasPrefix(info.Name(), ".") && path != config.path {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore patterns
		if len(gitignorePatterns) > 0 && shouldIgnore(path, gitignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply file/directory filters
		if config.filesOnly && info.IsDir() {
			return nil
		}
		if config.dirsOnly && !info.IsDir() {
			return nil
		}

		// Convert to relative path
		// If we have a working directory in context, calculate paths relative to it
		// Otherwise, calculate relative to the path being listed (original behavior)
		baseDir := config.path
		if cwd := ctx.Value("cwd"); cwd != nil {
			if cwdStr, ok := cwd.(string); ok && cwdStr != "" {
				// We have a session working directory, use it as base
				baseDir = cwdStr
			}
		}

		// Calculate relative path from base directory
		relPath, _ = filepath.Rel(baseDir, path)
		if relPath == "." {
			paths = append(paths, "./")
		} else {
			paths = append(paths, "./"+relPath)
		}
		count++

		// Check max results limit
		if config.maxResults > 0 && count >= config.maxResults {
			return fmt.Errorf("max_results_reached")
		}

		return nil
	})

	// Handle special case where we hit max results
	if err != nil && err.Error() == "max_results_reached" {
		err = nil // Not a real error
	}

	if err != nil && err != context.Canceled {
		return map[string]any{
			"success": false,
			"results": "",
			"error":   fmt.Sprintf("walk failed: %v", err),
		}, nil
	}

	result := strings.Join(paths, "\n")
	return map[string]any{
		"success": true,
		"results": result,
		"count":   count,
	}, nil
}

// FormatOutput formats file listing results for user display
func (l *LsTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	files, _ := result["results"].(string)
	errorMsg, _ := result["error"].(string)

	if !success {
		if errorMsg != "" {
			return fmt.Sprintf("**Failed to list files**: %s", errorMsg)
		}
		return "**Failed to list files**"
	}

	files = strings.TrimSpace(files)
	if files == "" {
		return "**Directory is empty**"
	}

	// Split files by newline and format as a list
	fileList := strings.Split(files, "\n")
	var formattedFiles []string
	for _, file := range fileList {
		file = strings.TrimSpace(file)
		if file != "" {
			// Add appropriate indicator based on file type
			if strings.HasSuffix(file, "/") {
				formattedFiles = append(formattedFiles, "[DIR]  "+file)
			} else {
				formattedFiles = append(formattedFiles, "[FILE] "+file)
			}
		}
	}

	return fmt.Sprintf("**Files in Directory**\n%s", strings.Join(formattedFiles, "\n"))
}

