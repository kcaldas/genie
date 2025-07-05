package tools

import (
	"fmt"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
)

// Registry defines the interface for managing tools
type Registry interface {
	// Register adds a tool to the registry
	Register(tool Tool) error
	
	// GetAll returns all registered tools
	GetAll() []Tool
	
	// Get returns a specific tool by name
	Get(name string) (Tool, bool)
	
	// Names returns all registered tool names
	Names() []string
}

// DefaultRegistry is a thread-safe implementation of Registry
type DefaultRegistry struct {
	tools map[string]Tool
	mutex sync.RWMutex
}

// NewRegistry creates a new empty tool registry
func NewRegistry() Registry {
	return &DefaultRegistry{
		tools: make(map[string]Tool),
	}
}

// NewDefaultRegistry creates a registry with tools configured for interactive use
func NewDefaultRegistry(eventBus events.EventBus, todoManager TodoManager) Registry {
	registry := NewRegistry()
	
	// Register all tools
	tools := []Tool{
		NewLsTool(eventBus),                            // List files with message support
		NewFindTool(eventBus),                          // Find files with message support
		NewReadFileTool(eventBus),                      // Read files with message support
		NewGrepTool(eventBus),                          // Search in files with message support
		NewBashTool(eventBus, eventBus, true),          // Bash with confirmation enabled
		NewWriteTool(eventBus, eventBus, true),         // Write files with diff preview enabled
		NewTodoReadTool(todoManager),                   // Todo read tool
		NewTodoWriteTool(todoManager),                  // Todo write tool
	}
	
	for _, tool := range tools {
		// Safe to ignore error since we control these tools
		_ = registry.Register(tool)
	}
	
	return registry
}

// Register adds a tool to the registry
func (r *DefaultRegistry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("cannot register nil tool")
	}
	
	declaration := tool.Declaration()
	if declaration == nil {
		return fmt.Errorf("tool declaration cannot be nil")
	}
	
	if declaration.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	// Check for duplicate registration
	if _, exists := r.tools[declaration.Name]; exists {
		return fmt.Errorf("tool with name '%s' already registered", declaration.Name)
	}
	
	r.tools[declaration.Name] = tool
	return nil
}

// GetAll returns all registered tools
func (r *DefaultRegistry) GetAll() []Tool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	
	return tools
}

// Get returns a specific tool by name
func (r *DefaultRegistry) Get(name string) (Tool, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	tool, exists := r.tools[name]
	return tool, exists
}

// Names returns all registered tool names
func (r *DefaultRegistry) Names() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	
	return names
}