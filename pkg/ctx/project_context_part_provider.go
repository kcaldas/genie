package ctx

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
)

// ProjectContextPartProvider manages project-specific context files (GENIE.md/CLAUDE.md)
type ProjectContextPartProvider interface {
	ContextPartProvider
}

// projectContextPartsProvider implements ProjectCtxManager
type projectContextPartsProvider struct {
	subscriber   events.Subscriber
	mu           sync.RWMutex
	contextFiles map[string]string // path -> content mapping
}

// NewProjectCtxManager creates a new project context manager
func NewProjectCtxManager(subscriber events.Subscriber) ProjectContextPartProvider {
	manager := &projectContextPartsProvider{
		subscriber:   subscriber,
		contextFiles: make(map[string]string),
	}

	// Subscribe to tool.executed events if subscriber is provided
	if subscriber != nil {
		subscriber.Subscribe("tool.executed", manager.handleToolExecutedEvent)
	}

	return manager
}

func (m *projectContextPartsProvider) SetTokenBudget(int) {}

// GetPart returns the concatenated project context
func (m *projectContextPartsProvider) GetPart(ctx context.Context) (ContextPart, error) {
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
	m.mu.RLock()
	for path, content := range m.contextFiles {
		if path != cwdContextPath { // Avoid duplicating CWD context
			contents = append(contents, content)
		}
	}
	m.mu.RUnlock()

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

// ClearPart is a no-op for project context (read-only)
func (m *projectContextPartsProvider) ClearPart() error {
	return nil
}

// getCachedCwdContext gets or reads CWD context file and caches it, returns content and path
func (m *projectContextPartsProvider) getCachedCwdContext(cwd string) (string, string) {
	// Check for GENIE.md in CWD
	genieMdPath := filepath.Join(cwd, "GENIE.md")

	// Check if already cached
	m.mu.RLock()
	content, exists := m.contextFiles[genieMdPath]
	m.mu.RUnlock()
	if exists {
		return content, genieMdPath
	}

	// Try to read GENIE.md
	fileContent, err := os.ReadFile(genieMdPath)
	if err == nil {
		contentStr := string(fileContent)
		m.mu.Lock()
		m.contextFiles[genieMdPath] = contentStr
		m.mu.Unlock()
		return contentStr, genieMdPath
	}

	// Check for CLAUDE.md if GENIE.md doesn't exist
	claudeMdPath := filepath.Join(cwd, "CLAUDE.md")

	// Check if already cached
	m.mu.RLock()
	content, exists = m.contextFiles[claudeMdPath]
	m.mu.RUnlock()
	if exists {
		return content, claudeMdPath
	}

	// Try to read CLAUDE.md
	fileContent, err = os.ReadFile(claudeMdPath)
	if err == nil {
		contentStr := string(fileContent)
		m.mu.Lock()
		m.contextFiles[claudeMdPath] = contentStr
		m.mu.Unlock()
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
func (m *projectContextPartsProvider) handleToolExecutedEvent(event any) {
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
	m.mu.RLock()
	_, exists := m.contextFiles[genieMdPath]
	m.mu.RUnlock()
	if exists {
		return
	}

	// Try to read GENIE.md
	content, err := os.ReadFile(genieMdPath)
	if err == nil {
		m.mu.Lock()
		m.contextFiles[genieMdPath] = string(content)
		m.mu.Unlock()
		return
	}

	// Look for CLAUDE.md if GENIE.md doesn't exist
	claudeMdPath := filepath.Join(fileDir, "CLAUDE.md")

	// Check if already cached
	m.mu.RLock()
	_, exists = m.contextFiles[claudeMdPath]
	m.mu.RUnlock()
	if exists {
		return
	}

	// Try to read CLAUDE.md
	content, err = os.ReadFile(claudeMdPath)
	if err == nil {
		m.mu.Lock()
		m.contextFiles[claudeMdPath] = string(content)
		m.mu.Unlock()
	}
}
