package ctx

import (
	"context"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/events"
)

// ProjectCtxManager manages project-specific context files (GENIE.md/CLAUDE.md)
type ProjectCtxManager interface {
	ContextPartProvider
}

// projectCtxManager implements ProjectCtxManager
type projectCtxManager struct {
	subscriber   events.Subscriber
	contextFiles map[string]string // path -> content mapping
}

// NewProjectCtxManager creates a new project context manager
func NewProjectCtxManager(subscriber events.Subscriber) ProjectCtxManager {
	manager := &projectCtxManager{
		subscriber:   subscriber,
		contextFiles: make(map[string]string),
	}

	// Subscribe to tool.executed events if subscriber is provided
	if subscriber != nil {
		subscriber.Subscribe("tool.executed", manager.handleToolExecutedEvent)
	}

	return manager
}

// GetContext returns the concatenated project context
func (m *projectCtxManager) GetContext(ctx context.Context) (ContextPart, error) {
	var contents []string
	var cwdContextPath string

	// Extract cwd from context and get its context file
	cwd, ok := ctx.Value("cwd").(string)
	if ok {
		content, contextPath := m.getCachedCwdContext(cwd)
		if content != "" {
			contents = append(contents, content)
			cwdContextPath = contextPath
		}
	}

	// Add all collected context files from tool executions (excluding CWD context)
	for path, content := range m.contextFiles {
		if path != cwdContextPath { // Avoid duplicating CWD context
			contents = append(contents, content)
		}
	}

	// If no content found, return empty ContextPart
	if len(contents) == 0 {
		return ContextPart{Key: "project", Content: ""}, nil
	}

	// Join with blank lines
	return ContextPart{
		Key:     "project",
		Content: joinWithBlankLines(contents),
	}, nil
}

// ClearContext is a no-op for project context (read-only)
func (m *projectCtxManager) ClearContext() error {
	return nil
}

// getCachedCwdContext gets or reads CWD context file and caches it, returns content and path
func (m *projectCtxManager) getCachedCwdContext(cwd string) (string, string) {
	// Check for GENIE.md in CWD
	genieMdPath := filepath.Join(cwd, "GENIE.md")

	// Check if already cached
	if content, exists := m.contextFiles[genieMdPath]; exists {
		return content, genieMdPath
	}

	// Try to read GENIE.md
	content, err := os.ReadFile(genieMdPath)
	if err == nil {
		contentStr := string(content)
		m.contextFiles[genieMdPath] = contentStr
		return contentStr, genieMdPath
	}

	// Check for CLAUDE.md if GENIE.md doesn't exist
	claudeMdPath := filepath.Join(cwd, "CLAUDE.md")

	// Check if already cached
	if content, exists := m.contextFiles[claudeMdPath]; exists {
		return content, claudeMdPath
	}

	// Try to read CLAUDE.md
	content, err = os.ReadFile(claudeMdPath)
	if err == nil {
		contentStr := string(content)
		m.contextFiles[claudeMdPath] = contentStr
		return contentStr, claudeMdPath
	}

	return "", ""
}

// joinWithBlankLines joins strings with blank lines between them
func joinWithBlankLines(contents []string) string {
	if len(contents) == 0 {
		return ""
	}
	if len(contents) == 1 {
		return contents[0]
	}

	result := contents[0]
	for i := 1; i < len(contents); i++ {
		result += "\n\n" + contents[i]
	}
	return result
}

// handleToolExecutedEvent handles tool.executed events
func (m *projectCtxManager) handleToolExecutedEvent(event any) {
	toolEvent, ok := event.(events.ToolExecutedEvent)
	if !ok {
		return
	}

	// Only handle readFile tool executions
	if toolEvent.ToolName != "readFile" {
		return
	}

	// Extract file path from parameters
	filePath, ok := toolEvent.Parameters["file_path"].(string)
	if !ok {
		return
	}

	// Get the directory of the file
	fileDir := filepath.Dir(filePath)

	// Look for GENIE.md in that directory
	genieMdPath := filepath.Join(fileDir, "GENIE.md")

	// Check if already cached
	if _, exists := m.contextFiles[genieMdPath]; exists {
		return
	}

	// Try to read GENIE.md
	content, err := os.ReadFile(genieMdPath)
	if err == nil {
		m.contextFiles[genieMdPath] = string(content)
		return
	}

	// Look for CLAUDE.md if GENIE.md doesn't exist
	claudeMdPath := filepath.Join(fileDir, "CLAUDE.md")

	// Check if already cached
	if _, exists := m.contextFiles[claudeMdPath]; exists {
		return
	}

	// Try to read CLAUDE.md
	content, err = os.ReadFile(claudeMdPath)
	if err == nil {
		m.contextFiles[claudeMdPath] = string(content)
	}
}
