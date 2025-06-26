package ctx

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/events"
)

type FileContextPartsProvider struct {
	eventBus     events.EventBus
	storedFiles  map[string]string // map[filePath]content
	orderedFiles []string          // ordered list of file paths (most recent first)
}

func NewFileContextPartsProvider(eventBus events.EventBus) *FileContextPartsProvider {
	provider := &FileContextPartsProvider{
		eventBus:     eventBus,
		storedFiles:  make(map[string]string),
		orderedFiles: make([]string, 0),
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

		// Store content
		p.storedFiles[filePath] = result

		// Update order
		// Check if file already exists in orderedFiles
		found := false
		for i, existingPath := range p.orderedFiles {
			if existingPath == filePath {
				// Move to front
				p.orderedFiles = append(p.orderedFiles[:i], p.orderedFiles[i+1:]...)
				p.orderedFiles = append([]string{filePath}, p.orderedFiles...)
				found = true
				break
			}
		}
		if !found {
			// Add to front if new
			p.orderedFiles = append([]string{filePath}, p.orderedFiles...)
		}
	}
}

// GetStoredFiles returns the map of stored file paths to content (for testing)
func (p *FileContextPartsProvider) GetStoredFiles() map[string]string {
	return p.storedFiles
}

// GetOrderedFiles returns the ordered list of file paths (for testing)
func (p *FileContextPartsProvider) GetOrderedFiles() []string {
	return p.orderedFiles
}

func (p *FileContextPartsProvider) GetPart(ctx context.Context) (ContextPart, error) {
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
	p.storedFiles = make(map[string]string)
	p.orderedFiles = make([]string, 0)
	return nil
}

