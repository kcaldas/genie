package prompts

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
	"gopkg.in/yaml.v2"
)

//go:embed prompts/*
var promptsFS embed.FS

// Loader defines how prompts are loaded
type Loader interface {
	LoadPrompt(promptName string) (ai.Prompt, error)
}


// DefaultLoader loads prompts from embedded file system and enhances them with tools
type DefaultLoader struct {
	Publisher   events.Publisher     // Event publisher for tool execution events
	promptCache map[string]ai.Prompt // Cache to store loaded prompts
	cacheMutex  sync.RWMutex         // Mutex to protect the cache map
}

// LoadPrompt loads a prompt from the embedded file system and enhances it with tools
func (l *DefaultLoader) LoadPrompt(promptName string) (ai.Prompt, error) {
	// First, check if the prompt is in the cache
	l.cacheMutex.RLock()
	prompt, exists := l.promptCache[promptName]
	l.cacheMutex.RUnlock()

	if exists {
		return prompt, nil
	}

	// Not in cache, load from embedded file system
	data, err := promptsFS.ReadFile("prompts/" + promptName + ".yaml")
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error reading embedded prompt file: %w", err)
	}

	var newPrompt ai.Prompt
	err = yaml.Unmarshal(data, &newPrompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error unmarshaling prompt: %w", err)
	}

	// Enhance the prompt with tools
	l.addTools(&newPrompt)

	// Store in cache
	l.cacheMutex.Lock()
	l.promptCache[promptName] = newPrompt
	l.cacheMutex.Unlock()

	return newPrompt, nil
}

// NewPromptLoader creates a new PromptLoader using embedded prompts
func NewPromptLoader(publisher events.Publisher) Loader {
	return &DefaultLoader{
		Publisher:   publisher,
		promptCache: make(map[string]ai.Prompt),
	}
}

// CacheSize returns the number of prompts in the cache (for testing and observability)
func (l *DefaultLoader) CacheSize() int {
	l.cacheMutex.RLock()
	defer l.cacheMutex.RUnlock()
	return len(l.promptCache)
}

// FileLoader is the file-based implementation of Loader
type FileLoader struct {
	PromptsPath string
}

// LoadPrompt loads a prompt from disk
func (l *FileLoader) LoadPrompt(promptName string) (ai.Prompt, error) {
	data, err := os.ReadFile(filepath.Join(l.PromptsPath, promptName+".yaml"))
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error reading prompt file: %w", err)
	}

	var prompt ai.Prompt
	err = yaml.Unmarshal(data, &prompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error unmarshaling prompt: %w", err)
	}

	return prompt, nil
}

// addTools adds available tools to the prompt
func (l *DefaultLoader) addTools(prompt *ai.Prompt) {
	// Create specific tools
	toolsList := []tools.Tool{
		tools.NewLsTool(),       // List files
		tools.NewFindTool(),     // Find files
		tools.NewCatTool(),      // Read files
		tools.NewGrepTool(),     // Search in files
		tools.NewGitStatusTool(), // Git status
		tools.NewBashTool(),     // Fallback for other commands
	}
	
	// Initialize Functions slice if nil
	if prompt.Functions == nil {
		prompt.Functions = []*ai.FunctionDeclaration{}
	}
	
	// Initialize Handlers map if nil
	if prompt.Handlers == nil {
		prompt.Handlers = make(map[string]ai.HandlerFunc)
	}
	
	// Add all tool declarations and handlers
	for _, tool := range toolsList {
		declaration := tool.Declaration()
		prompt.Functions = append(prompt.Functions, declaration)
		
		// Wrap the handler to publish events when tools are executed
		originalHandler := tool.Handler()
		wrappedHandler := l.wrapHandlerWithEvents(declaration.Name, originalHandler)
		prompt.Handlers[declaration.Name] = wrappedHandler
	}
}

// wrapHandlerWithEvents wraps a tool handler to publish events when executed
func (l *DefaultLoader) wrapHandlerWithEvents(toolName string, handler ai.HandlerFunc) ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Execute the original handler
		result, err := handler(ctx, params)
		
		// Create a message based on the tool and result
		var message string
		if err != nil {
			message = fmt.Sprintf("Failed: %v", err)
		} else {
			message = "Executed"
		}
		
		// Publish the tool execution event
		if l.Publisher != nil {
			// Try to get session ID from context
			sessionID := "unknown"
			if ctx != nil {
				if id, ok := ctx.Value("sessionID").(string); ok && id != "" {
					sessionID = id
				}
			}
			
			event := events.ToolExecutedEvent{
				SessionID:  sessionID,
				ToolName:   toolName,
				Parameters: params,
				Message:    message,
			}
			l.Publisher.Publish(event.Topic(), event)
		}
		
		return result, err
	}
}
