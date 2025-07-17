package ctx

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
)

type FileContextPartsProvider struct {
	eventBus     events.EventBus
	storedFiles  map[string]string // map[filePath]content
	orderedFiles []string          // ordered list of file paths (most recent first)
	fileIndexes  map[string]int    // map[filePath]index in orderedFiles for O(1) lookup
	mu           sync.RWMutex      // protects all maps and slices
}

func NewFileContextPartsProvider(eventBus events.EventBus) *FileContextPartsProvider {
	provider := &FileContextPartsProvider{
		eventBus:     eventBus,
		storedFiles:  make(map[string]string),
		orderedFiles: make([]string, 0),
		fileIndexes:  make(map[string]int),
	}

	if eventBus != nil {
		eventBus.Subscribe("tool.executed", provider.handleToolExecutedEvent)
	}
	return provider
}

func (p *FileContextPartsProvider) handleToolExecutedEvent(event interface{}) {
	toolEvent, ok := event.(events.ToolExecutedEvent)
	if !ok {
		return
	}

	if toolEvent.ToolName == "readFile" {
		filePath, ok := toolEvent.Parameters["file_path"].(string)
		if !ok {
			return
		}
		result, ok := toolEvent.Result["results"].(string)
		if !ok {
			return
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		// Store content
		p.storedFiles[filePath] = result

		// Update order
		if idx, found := p.fileIndexes[filePath]; found {
			// File already exists, remove it from its current position
			p.orderedFiles = append(p.orderedFiles[:idx], p.orderedFiles[idx+1:]...)
			// Re-index elements after the removed one
			for i := idx; i < len(p.orderedFiles); i++ {
				p.fileIndexes[p.orderedFiles[i]] = i
			}
		}

		// Add to front of orderedFiles
		p.orderedFiles = append([]string{filePath}, p.orderedFiles...)
		// Update indexes for all elements
		for i, path := range p.orderedFiles {
			p.fileIndexes[path] = i
		}
	}
}

// GetStoredFiles returns the map of stored file paths to content (for testing)
func (p *FileContextPartsProvider) GetStoredFiles() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Return a copy to avoid data races
	result := make(map[string]string)
	for k, v := range p.storedFiles {
		result[k] = v
	}
	return result
}

// GetOrderedFiles returns the ordered list of file paths (for testing)
func (p *FileContextPartsProvider) GetOrderedFiles() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Return a copy to avoid data races
	result := make([]string, len(p.orderedFiles))
	copy(result, p.orderedFiles)
	return result
}

func (p *FileContextPartsProvider) GetPart(ctx context.Context) (ContextPart, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	var parts []string
	for _, filePath := range p.orderedFiles {
		content, ok := p.storedFiles[filePath]
		if !ok {
			// This should ideally not happen if logic is correct, but handle defensively
			continue
		}
		parts = append(parts, fmt.Sprintf("File: %s\n```\n%s\n```", filePath, content))
	}

	return ContextPart{
		Key:     "files",
		Content: strings.Join(parts, "\n\n"),
	}, nil
}

func (p *FileContextPartsProvider) ClearPart() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.storedFiles = make(map[string]string)
	p.orderedFiles = make([]string, 0)
	p.fileIndexes = make(map[string]int) // Clear file indexes as well
	return nil
}
