package prompts

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
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
	Publisher    events.Publisher     // Event publisher for tool execution events
	ToolRegistry tools.Registry       // Tool registry for getting available tools
	Config       config.Manager       // Configuration manager for model defaults
	promptCache  map[string]ai.Prompt // Cache to store loaded prompts
	cacheMutex   sync.RWMutex         // Mutex to protect the cache map
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

	// Apply default model configuration for any missing fields
	l.applyModelDefaults(&newPrompt)
	
	// Enhance the prompt with tools
	err = l.addTools(&newPrompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("failed to add tools to prompt: %w", err)
	}

	// Store in cache
	l.cacheMutex.Lock()
	l.promptCache[promptName] = newPrompt
	l.cacheMutex.Unlock()

	return newPrompt, nil
}

// NewPromptLoader creates a new PromptLoader using embedded prompts
func NewPromptLoader(publisher events.Publisher, toolRegistry tools.Registry) Loader {
	return &DefaultLoader{
		Publisher:    publisher,
		ToolRegistry: toolRegistry,
		Config:       config.NewConfigManager(),
		promptCache:  make(map[string]ai.Prompt),
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

// applyModelDefaults applies default model configuration for any missing fields
func (l *DefaultLoader) applyModelDefaults(prompt *ai.Prompt) {
	modelConfig := l.Config.GetModelConfig()
	
	// Apply defaults only if fields are empty/zero
	if prompt.ModelName == "" {
		prompt.ModelName = modelConfig.ModelName
	}
	if prompt.MaxTokens == 0 {
		prompt.MaxTokens = modelConfig.MaxTokens
	}
	if prompt.Temperature == 0 {
		prompt.Temperature = modelConfig.Temperature
	}
	if prompt.TopP == 0 {
		prompt.TopP = modelConfig.TopP
	}
}

// addTools adds required tools to the prompt
func (l *DefaultLoader) addTools(prompt *ai.Prompt) error {
	// Only add tools if RequiredTools is explicitly specified
	if prompt.RequiredTools == nil {
		// No required_tools field in YAML = no tools
		return nil
	}
	
	var toolsList []tools.Tool
	var missingTools []string
	
	// Use existing registry Get method (already O(1) map lookup)
	for _, toolName := range prompt.RequiredTools {
		if tool, exists := l.ToolRegistry.Get(toolName); exists {
			toolsList = append(toolsList, tool)
		} else {
			missingTools = append(missingTools, toolName)
		}
	}
	
	if len(missingTools) > 0 {
		return fmt.Errorf("missing required tools: %v", missingTools)
	}
	
	// Initialize Functions slice if nil
	if prompt.Functions == nil {
		prompt.Functions = []*ai.FunctionDeclaration{}
	}
	
	// Initialize Handlers map if nil
	if prompt.Handlers == nil {
		prompt.Handlers = make(map[string]ai.HandlerFunc)
	}
	
	// Add tools to prompt
	for _, tool := range toolsList {
		declaration := tool.Declaration()
		prompt.Functions = append(prompt.Functions, declaration)
		
		// Wrap handler with events
		originalHandler := tool.Handler()
		wrappedHandler := l.wrapHandlerWithEvents(declaration.Name, originalHandler)
		prompt.Handlers[declaration.Name] = wrappedHandler
	}
	
	return nil
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
			// Try to get session ID and execution ID from context
			sessionID := "unknown"
			executionID := "unknown"
			if ctx != nil {
				if id, ok := ctx.Value("sessionID").(string); ok && id != "" {
					sessionID = id
				}
				if id, ok := ctx.Value("executionID").(string); ok && id != "" {
					executionID = id
				}
			}
			
			event := events.ToolExecutedEvent{
				ExecutionID: executionID,
				SessionID:   sessionID,
				ToolName:    toolName,
				Parameters:  params,
				Message:     message,
			}
			l.Publisher.Publish(event.Topic(), event)
		}
		
		return result, err
	}
}
