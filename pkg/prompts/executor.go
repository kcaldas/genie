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

// Executor defines the behavior of a service that generates a response for a given prompt and highlight in context
type Executor interface {
	Execute(ctx context.Context, promptName string, debug bool, promptData ...ai.Attr) (string, error)
	ExecuteWithSchema(ctx context.Context, promptName string, schema *ai.Schema, promptData ...ai.Attr) (string, error)
	CacheSize() int // For testing purposes
}

// Loader loads prompts from embedded file system
type DefaultLoader struct{}

// LoadPrompt loads a prompt from the embedded file system
func (l *DefaultLoader) LoadPrompt(promptName string) (ai.Prompt, error) {
	data, err := promptsFS.ReadFile("prompts/" + promptName + ".yaml")
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error reading embedded prompt file: %w", err)
	}

	var prompt ai.Prompt
	err = yaml.Unmarshal(data, &prompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error unmarshaling prompt: %w", err)
	}

	return prompt, nil
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

// Executor represents a service that generates a response for a given prompt and highlight in context
type DefaultExecutor struct {
	Gen         ai.Gen
	Loader      Loader
	Publisher   events.Publisher     // Event publisher for tool execution events
	promptCache map[string]ai.Prompt // Cache to store loaded prompts
	cacheMutex  sync.RWMutex         // Mutex to protect the cache map
}

// CacheSize returns the number of prompts in the cache (for testing purposes)
func (s *DefaultExecutor) CacheSize() int {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return len(s.promptCache)
}

// NewExecutor creates a new DefaultExecutor with embedded prompts
func NewExecutor(gen ai.Gen, publisher events.Publisher) Executor {
	loader := &DefaultLoader{}

	return &DefaultExecutor{
		Gen:         gen,
		Loader:      loader,
		Publisher:   publisher,
		promptCache: make(map[string]ai.Prompt),
	}
}

// NewFileBasedExecutor creates a new DefaultExecutor with a file-based loader
func NewFileBasedExecutor(gen ai.Gen, publisher events.Publisher, promptsPath string) Executor {
	loader := &FileLoader{
		PromptsPath: promptsPath,
	}

	return &DefaultExecutor{
		Gen:         gen,
		Loader:      loader,
		Publisher:   publisher,
		promptCache: make(map[string]ai.Prompt),
	}
}

// NewWithLoader creates a DefaultExecutor with a custom loader (useful for testing)
func NewWithLoader(gen ai.Gen, publisher events.Publisher, loader Loader) Executor {
	return &DefaultExecutor{
		Gen:         gen,
		Loader:      loader,
		Publisher:   publisher,
		promptCache: make(map[string]ai.Prompt),
	}
}

// getPrompt loads a prompt from cache or uses the loader if not cached
func (s *DefaultExecutor) getPrompt(promptName string) (ai.Prompt, error) {
	// First, check if the prompt is in the cache
	s.cacheMutex.RLock()
	prompt, exists := s.promptCache[promptName]
	s.cacheMutex.RUnlock()

	if exists {
		return prompt, nil
	}

	// Not in cache, use the prompt loader
	newPrompt, err := s.Loader.LoadPrompt(promptName)
	if err != nil {
		return ai.Prompt{}, err
	}

	// Store in cache
	s.cacheMutex.Lock()
	s.promptCache[promptName] = newPrompt
	s.cacheMutex.Unlock()

	return newPrompt, nil
}

// Execute generates a response for the given prompt and highlight in context
func (s *DefaultExecutor) Execute(ctx context.Context, promptName string, debug bool, promptData ...ai.Attr) (string, error) {
	prompt, err := s.getPrompt(promptName)
	if err != nil {
		return "", err
	}

	// Add bash tool to the prompt
	s.addTools(&prompt)

	result, err := s.Gen.GenerateContentAttr(ctx, prompt, debug, promptData)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}
	result = ai.RemoveSurroundingMarkdown(result)

	return result, nil
}

// addTools adds available tools to the prompt
func (s *DefaultExecutor) addTools(prompt *ai.Prompt) {
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
		wrappedHandler := s.wrapHandlerWithEvents(declaration.Name, originalHandler)
		prompt.Handlers[declaration.Name] = wrappedHandler
	}
}

// wrapHandlerWithEvents wraps a tool handler to publish events when executed
func (s *DefaultExecutor) wrapHandlerWithEvents(toolName string, handler ai.HandlerFunc) ai.HandlerFunc {
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
		if s.Publisher != nil {
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
			s.Publisher.Publish(event.Topic(), event)
		}
		
		return result, err
	}
}

func (s *DefaultExecutor) ExecuteWithSchema(ctx context.Context, promptName string, schema *ai.Schema, promptData ...ai.Attr) (string, error) {
	prompt, err := s.getPrompt(promptName)
	if err != nil {
		return "", err
	}

	// Make a copy of the prompt and add the schema
	promptCopy := prompt
	promptCopy.ResponseSchema = schema
	
	// Add bash tool to the prompt
	s.addTools(&promptCopy)

	result, err := s.Gen.GenerateContentAttr(ctx, promptCopy, true, promptData)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}
	result = ai.RemoveSurroundingMarkdown(result)

	return result, nil
}