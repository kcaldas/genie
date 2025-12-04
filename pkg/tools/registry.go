package tools

import (
	"fmt"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/skills"
)

// SkillManager is an alias to avoid repeating the import
type SkillManager = skills.SkillManager

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

	// RegisterToolSet registers a group of tools under a toolSet name
	RegisterToolSet(setName string, tools []Tool) error

	// GetToolSet returns all tools in a toolSet
	GetToolSet(setName string) ([]Tool, bool)

	// GetToolSetNames returns all registered toolSet names
	GetToolSetNames() []string

	// Init initializes the registry with the given working directory.
	// This triggers MCP discovery and connection for the specified directory.
	Init(workingDir string) error
}

// DefaultRegistry is a thread-safe implementation of Registry
type DefaultRegistry struct {
	tools       map[string]Tool
	toolSets    map[string][]Tool
	mutex       sync.RWMutex
	mcpClient   MCPClient
	initialized bool
}

// NewRegistry creates a new empty tool registry
func NewRegistry() Registry {
	return &DefaultRegistry{
		tools:    make(map[string]Tool),
		toolSets: make(map[string][]Tool),
	}
}

// NewDefaultRegistry creates a registry with tools configured for interactive use
func NewDefaultRegistry(eventBus events.EventBus, todoManager TodoManager, skillManager SkillManager) Registry {
	registry := NewRegistry()

	// Register all tools
	tools := []Tool{
		NewLsTool(eventBus),                    // List files with message support
		NewFindTool(eventBus),                  // Find files with message support
		NewReadFileTool(eventBus),              // Read files with message support
		NewViewDocumentTool(eventBus),          // Inspect PDF documents
		NewViewImageTool(eventBus),             // Inspect images within the workspace
		NewGrepTool(eventBus),                  // Search in files with message support
		NewBashTool(eventBus, eventBus, false), // Bash with confirmation always disabled. The LLM will decide
		NewWriteTool(eventBus, eventBus, true), // Write files with diff preview enabled
		NewTodoWriteTool(todoManager),          // Todo write tool
		NewThinkingTool(eventBus),              // Thinking tool
		NewTaskTool(eventBus),                  // Task tool for subprocess research
	}

	// Add Skill tool if skill manager is available
	if skillManager != nil {
		tools = append(tools, NewSkillTool(skillManager, eventBus))
	}

	for _, tool := range tools {
		// Safe to ignore error since we control these tools
		_ = registry.Register(tool)
	}

	// Register "essentials" toolset
	essentialsTools := []Tool{
		NewTodoWriteTool(todoManager),
		NewThinkingTool(eventBus),
	}

	// Add Skill tool to essentials if skillManager is available
	if skillManager != nil {
		essentialsTools = append(essentialsTools, NewSkillTool(skillManager, eventBus))
	}

	_ = registry.RegisterToolSet("essentials", essentialsTools) // Safe to ignore error as these are internal tools

	return registry
}

// NewRegistryWithMCP creates a registry with default tools and stores the MCP client for lazy initialization.
// MCP tools are not loaded until Init(workingDir) is called, which allows proper directory scoping.
func NewRegistryWithMCP(eventBus events.EventBus, todoManager TodoManager, skillManager SkillManager, mcpClient MCPClient) Registry {
	// Start with default registry (returns *DefaultRegistry)
	baseRegistry := NewDefaultRegistry(eventBus, todoManager, skillManager)

	// Store MCP client for lazy initialization
	if defaultReg, ok := baseRegistry.(*DefaultRegistry); ok {
		defaultReg.mcpClient = mcpClient
	}

	return baseRegistry
}

// MCPClient interface for dependency injection (avoids circular imports)
type MCPClient interface {
	GetTools() []Tool
	GetToolsByServer() map[string][]Tool
	// Init initializes the MCP client with the given working directory.
	// This discovers MCP config and connects to servers.
	Init(workingDir string) error
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

// RegisterToolSet registers a group of tools under a toolSet name
func (r *DefaultRegistry) RegisterToolSet(setName string, tools []Tool) error {
	if setName == "" {
		return fmt.Errorf("toolSet name cannot be empty")
	}

	if len(tools) == 0 {
		return fmt.Errorf("cannot register empty toolSet")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check for duplicate toolSet registration
	if _, exists := r.toolSets[setName]; exists {
		return fmt.Errorf("toolSet with name '%s' already registered", setName)
	}

	// Validate all tools in the set
	for i, tool := range tools {
		if tool == nil {
			return fmt.Errorf("cannot register nil tool at index %d in toolSet '%s'", i, setName)
		}

		declaration := tool.Declaration()
		if declaration == nil {
			return fmt.Errorf("tool declaration cannot be nil at index %d in toolSet '%s'", i, setName)
		}

		if declaration.Name == "" {
			return fmt.Errorf("tool name cannot be empty at index %d in toolSet '%s'", i, setName)
		}
	}

	// Register the toolSet
	r.toolSets[setName] = make([]Tool, len(tools))
	copy(r.toolSets[setName], tools)

	return nil
}

// GetToolSet returns all tools in a toolSet
func (r *DefaultRegistry) GetToolSet(setName string) ([]Tool, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tools, exists := r.toolSets[setName]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	result := make([]Tool, len(tools))
	copy(result, tools)
	return result, true
}

// GetToolSetNames returns all registered toolSet names
func (r *DefaultRegistry) GetToolSetNames() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	names := make([]string, 0, len(r.toolSets))
	for name := range r.toolSets {
		names = append(names, name)
	}

	return names
}

// Init initializes the registry by initializing the MCP client with the working directory.
// This triggers MCP config discovery and server connections for the specified directory.
func (r *DefaultRegistry) Init(workingDir string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.initialized {
		return nil // Already initialized
	}

	// Initialize MCP client if available
	if r.mcpClient != nil {
		if err := r.mcpClient.Init(workingDir); err != nil {
			return fmt.Errorf("failed to initialize MCP client: %w", err)
		}

		// Register MCP tools now that client is initialized
		mcpTools := r.mcpClient.GetTools()
		for _, tool := range mcpTools {
			// Register MCP tools, but don't fail if there are conflicts
			r.tools[tool.Declaration().Name] = tool
		}

		// Register toolSets for each MCP server
		toolsByServer := r.mcpClient.GetToolsByServer()
		for serverName, serverTools := range toolsByServer {
			if len(serverTools) > 0 {
				r.toolSets[serverName] = serverTools
			}
		}
	}

	r.initialized = true
	return nil
}
