// Package prompts provides prompt loading functionality for Genie.
//
// This package supports loading prompts from arbitrary file paths with
// caching, model configuration defaults, and tool enhancement.
//
// The DefaultLoader provides:
// - File-based prompt loading with permanent caching
// - Automatic model configuration defaults (model name, tokens, temperature, etc.)
// - Tool enhancement based on required_tools in prompt YAML
// - Event wrapping for tool execution
//
// Prompts are loaded by the persona system from:
// - Internal personas: embedded in pkg/persona/personas/{name}/prompt.yaml
// - User personas: ~/.genie/personas/{name}/prompt.yaml
// - Project personas: $cwd/.genie/personas/{name}/prompt.yaml
package prompts

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
	"strings" // Added for string manipulation
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
	"gopkg.in/yaml.v2"
)

// Loader defines how prompts are loaded
type Loader interface {
	LoadPromptFromFS(filesystem fs.FS, filePath string) (ai.Prompt, error)
}

// DefaultLoader loads prompts from file paths and enhances them with tools
type DefaultLoader struct {
	Publisher    events.Publisher     // Event publisher for tool execution events
	ToolRegistry tools.Registry       // Tool registry for getting available tools
	Config       config.Manager       // Configuration manager for model defaults
	promptCache  map[string]ai.Prompt // Cache to store loaded prompts by file path
	cacheMutex   sync.RWMutex         // Mutex to protect the cache map
}

// LoadPromptFromFS loads a prompt from a filesystem (regular or embedded) and enhances it with tools
func (l *DefaultLoader) LoadPromptFromFS(filesystem fs.FS, filePath string) (ai.Prompt, error) {
	// Create cache key combining filesystem type and path
	cacheKey := fmt.Sprintf("%T:%s", filesystem, filePath)

	// Check cache first
	l.cacheMutex.RLock()
	if cachedPrompt, found := l.promptCache[cacheKey]; found {
		l.cacheMutex.RUnlock()
		return cachedPrompt, nil
	}
	l.cacheMutex.RUnlock()

	// Read file from filesystem
	data, err := fs.ReadFile(filesystem, filePath)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error reading prompt file %s: %w", filePath, err)
	}

	var newPrompt ai.Prompt
	err = yaml.Unmarshal(data, &newPrompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error unmarshaling prompt from %s: %w", filePath, err)
	}

	// Apply default model configuration for any missing fields
	l.ApplyModelDefaults(&newPrompt)

	// Enhance the prompt with tools
	err = l.AddTools(&newPrompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("failed to add tools to prompt from %s: %w", filePath, err)
	}

	// Cache the enhanced prompt
	l.cacheMutex.Lock()
	l.promptCache[cacheKey] = newPrompt
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

// ApplyModelDefaults applies default model configuration for any missing fields
func (l *DefaultLoader) ApplyModelDefaults(prompt *ai.Prompt) {
	modelConfig := l.Config.GetModelConfig()

	if prompt.LLMProvider == "" {
		prompt.LLMProvider = strings.ToLower(l.Config.GetStringWithDefault("GENIE_LLM_PROVIDER", "genai"))
	}

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

// AddTools adds required tools to the prompt
func (l *DefaultLoader) AddTools(prompt *ai.Prompt) error {
	// Only add tools if RequiredTools is explicitly specified
	if prompt.RequiredTools == nil {
		// No required_tools field in YAML = no tools
		return nil
	}

	var toolsList []tools.Tool
	var missingTools []string

	// Use existing registry Get method (already O(1) map lookup)
	for _, toolName := range prompt.RequiredTools {
		// Check if this is a toolSet reference (starts with @)
		if strings.HasPrefix(toolName, "@") {
			setName := strings.TrimPrefix(toolName, "@")
			if setTools, exists := l.ToolRegistry.GetToolSet(setName); exists {
				toolsList = append(toolsList, setTools...)
			} else {
				missingTools = append(missingTools, toolName)
			}
		} else {
			// Regular tool lookup
			if tool, exists := l.ToolRegistry.Get(toolName); exists {
				toolsList = append(toolsList, tool)
			} else {
				missingTools = append(missingTools, toolName)
			}
		}
	}

	if len(missingTools) > 0 {
		availableTools := slices.Collect(func(yield func(string) bool) {
			for _, t := range l.ToolRegistry.GetAll() {
				if !yield(t.Declaration().Name) {
					return
				}
			}
		})

		availableToolSets := slices.Collect(func(yield func(string) bool) {
			for _, setName := range l.ToolRegistry.GetToolSetNames() {
				if !yield("@" + setName) {
					return
				}
			}
		})

		return fmt.Errorf("missing required tools: %v, available tools: %v, available toolSets: %v", missingTools, availableTools, availableToolSets)
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
			executionID := "unknown"
			if ctx != nil {
				if id, ok := ctx.Value("executionID").(string); ok && id != "" {
					executionID = id
				}
			}

			// Filter out parameters starting with "_"
			filteredParams := make(map[string]any)
			for k, v := range params {
				if !strings.HasPrefix(k, "_") {
					filteredParams[k] = v
				}
			}

			event := events.ToolExecutedEvent{
				ExecutionID: executionID,
				ToolName:    toolName,
				Parameters:  filteredParams, // Use filtered parameters
				Message:     message,
				Result:      result,
			}
			l.Publisher.Publish(event.Topic(), event)
		}

		return result, err
	}
}
