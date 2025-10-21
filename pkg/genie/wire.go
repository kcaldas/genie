//go:build wireinject

package genie

import (
	"strings"

	"github.com/google/wire"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/anthropic"
	"github.com/kcaldas/genie/pkg/llm/genai"
	"github.com/kcaldas/genie/pkg/llm/multiplexer"
	"github.com/kcaldas/genie/pkg/llm/ollama"
	"github.com/kcaldas/genie/pkg/llm/openai"
	"github.com/kcaldas/genie/pkg/mcp"
	"github.com/kcaldas/genie/pkg/persona"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/tools"
)

// Shared event bus instance
var eventBus = events.NewEventBus()

// Wire providers for event bus system

func ProvideEventBus() events.EventBus {
	return eventBus
}

func ProvidePublisher() events.Publisher {
	return eventBus
}

func ProvideSubscriber() events.Subscriber {
	return eventBus
}

// ProvideTodoManager provides a shared todo manager instance
func ProvideTodoManager() tools.TodoManager {
	return tools.NewTodoManager()
}

// ProvideMCPClient provides an MCP client
func ProvideMCPClient() (tools.MCPClient, error) {
	client, err := mcp.NewMCPClientFromConfig()
	if err != nil {
		return nil, err
	}
	return client, nil
}

// ProvideToolRegistry provides a tool registry with interactive tools and MCP tools
func ProvideToolRegistry() (tools.Registry, error) {
	wire.Build(ProvideEventBus, ProvideTodoManager, ProvideMCPClient, tools.NewRegistryWithMCP)
	return nil, nil
}

// ProvideToolRegistryWithOptions provides a tool registry with optional customization
// This provider checks the GenieOptions and:
// - Uses CustomRegistry if provided
// - Uses CustomRegistryFactory if provided
// - Otherwise creates a default registry and adds CustomTools if provided
func ProvideToolRegistryWithOptions(options *GenieOptions) (tools.Registry, error) {
	wire.Build(
		ProvideEventBus,
		ProvideTodoManager,
		ProvideMCPClient,
		newRegistryWithOptions,
	)
	return nil, nil
}

// newRegistryWithOptions is a provider function that creates a registry based on options
func newRegistryWithOptions(
	eventBus events.EventBus,
	todoManager tools.TodoManager,
	mcpClient tools.MCPClient,
	options *GenieOptions,
) (tools.Registry, error) {
	// If a custom registry is provided, use it directly
	if options.CustomRegistry != nil {
		return options.CustomRegistry, nil
	}

	// If a custom factory is provided, use it
	if options.CustomRegistryFactory != nil {
		return options.CustomRegistryFactory(eventBus, todoManager), nil
	}

	// Otherwise, create default registry with MCP tools
	registry := tools.NewRegistryWithMCP(eventBus, todoManager, mcpClient)

	// Add any custom tools to the default registry
	for _, tool := range options.CustomTools {
		if err := registry.Register(tool); err != nil {
			// Log the error but continue - we don't want to fail initialization
			// just because a custom tool has a naming conflict
			_ = err
		}
	}

	return registry, nil
}

// ProvideOutputFormatter provides a tool output formatter
func ProvideOutputFormatter() (tools.OutputFormatter, error) {
	wire.Build(ProvideToolRegistry, tools.NewOutputFormatter)
	return nil, nil
}

// Wire injectors for singleton managers

func ProvideProjectCtxManager() ctx.ProjectContextPartProvider {
	wire.Build(ProvideSubscriber, ctx.NewProjectCtxManager)
	return nil
}

func ProvideChatCtxManager() ctx.ChatContextPartProvider {
	wire.Build(ProvideEventBus, ctx.NewChatCtxManager)
	return nil
}

func ProvideFileContextPartsProvider() *ctx.FileContextPartsProvider {
	wire.Build(ProvideEventBus, ctx.NewFileContextPartsProvider)
	return nil
}

func ProvideTodoContextPartsProvider() *ctx.TodoContextPartProvider {
	wire.Build(ProvideEventBus, ctx.NewTodoContextPartProvider)
	return nil
}

func ProvideContextRegistry() *ctx.ContextPartProviderRegistry {
	// Create registry
	registry := ctx.NewContextPartProviderRegistry()

	// Get managers (they now implement ContextPartProvider directly)
	projectManager := ProvideProjectCtxManager()
	chatManager := ProvideChatCtxManager()
	fileProvider := ProvideFileContextPartsProvider()
	todoProvider := ProvideTodoContextPartsProvider()

	// Register managers directly
	registry.Register(projectManager)
	registry.Register(chatManager)
	registry.Register(fileProvider)
	registry.Register(todoProvider)

	return registry
}

func ProvideContextManager() ctx.ContextManager {
	wire.Build(ProvideContextRegistry, ctx.NewContextManager)
	return nil
}

func ProvideSessionManager() SessionManager {
	wire.Build(ProvidePublisher, NewSessionManager)
	return nil
}

// ProvideGen is an injector function - Wire will generate the implementation
func ProvideGen() (ai.Gen, error) {
	wire.Build(
		ProvideAIGenWithCapture,
		ProvideConfigManager,
	)
	return nil, nil
}

// ProvideAIGenWithCapture creates the AI Gen with optional capture middleware
func ProvideAIGenWithCapture(configManager config.Manager) (ai.Gen, error) {
	provider := strings.ToLower(configManager.GetStringWithDefault("GENIE_LLM_PROVIDER", "genai"))

	factories := map[string]multiplexer.Factory{
		"genai":     func() (ai.Gen, error) { return genai.NewClient(eventBus) },
		"openai":    func() (ai.Gen, error) { return openai.NewClient(eventBus) },
		"anthropic": func() (ai.Gen, error) { return anthropic.NewClient(eventBus) },
		"ollama":    func() (ai.Gen, error) { return ollama.NewClient(eventBus) },
	}

	aliases := map[string]string{
		"gemini":           "genai",
		"google":           "genai",
		"vertex":           "genai",
		"openai-chat":      "openai",
		"claude":           "anthropic",
		"anthropic-claude": "anthropic",
	}

	muxClient, err := multiplexer.NewClient(provider, factories, aliases)
	if err != nil {
		return nil, err
	}

	if err := muxClient.WarmUp(provider); err != nil {
		return nil, err
	}

	baseGen := ai.Gen(muxClient)
	captureProvider := muxClient.DefaultProvider()

	// Get capture configuration from environment
	captureConfig := ai.GetCaptureConfigFromEnv(captureProvider)

	// Wrap with capture middleware if enabled
	if captureConfig.Enabled {
		baseGen = ai.NewCaptureMiddleware(baseGen, captureConfig)
	}

	// Get retry configuration from environment
	retryConfig := ai.GetRetryConfigFromEnv(configManager)

	// Wrap with retry middleware if enabled
	if retryConfig.Enabled {
		return ai.NewRetryMiddleware(baseGen, retryConfig), nil
	}

	return baseGen, nil
}

// ProvidePromptLoader is an injector function - Wire will generate the implementation
func ProvidePromptLoader() (prompts.Loader, error) {
	wire.Build(ProvidePublisher, ProvideToolRegistry, prompts.NewPromptLoader)
	return nil, nil
}

// ProvideConfigManager provides a configuration manager
func ProvideConfigManager() config.Manager {
	return config.NewConfigManager()
}

// ProvidePersonaPromptFactory provides the persona-aware prompt factory
func ProvidePersonaPromptFactory() (persona.PersonaAwarePromptFactory, error) {
	wire.Build(ProvidePromptLoader, persona.NewPersonaPromptFactory)
	return nil, nil
}

// ProviderPromptRunner provides the prompt runner
func ProvidePromptRunner() (PromptRunner, error) {
	wire.Build(ProvideGen, wire.Value(false), NewDefaultPromptRunner)
	return nil, nil
}

// ProvidePersonaManager provides the persona manager
func ProvidePersonaManager() (persona.PersonaManager, error) {
	wire.Build(ProvidePersonaPromptFactory, ProvideConfigManager, persona.NewDefaultPersonaManager)
	return nil, nil
}

// ProvideGenie provides a complete Genie instance using Wire
func ProvideGenie() (Genie, error) {
	wire.Build(
		ProvidePromptRunner,

		// Manager dependencies
		ProvideSessionManager,
		ProvideContextManager,

		// Event bus dependency
		ProvideEventBus,

		// Tool output formatter dependency
		ProvideOutputFormatter,

		// PersonaManager dependency
		ProvidePersonaManager,

		// Configuration dependency
		ProvideConfigManager,

		// Genie factory function
		newGenieCore,
	)
	return nil, nil
}

// ProvideGenieWithOptions provides a complete Genie instance with custom options
func ProvideGenieWithOptions(options *GenieOptions) (Genie, error) {
	wire.Build(
		// Use the options-aware tool registry provider
		ProvideToolRegistryWithOptions,

		// Output formatter that depends on the registry
		tools.NewOutputFormatter,

		// Prompt loader that depends on the registry
		ProvidePublisher,
		prompts.NewPromptLoader,

		// Persona system
		persona.NewPersonaPromptFactory,
		ProvideConfigManager,
		persona.NewDefaultPersonaManager,

		// Prompt runner
		ProvideGen,
		wire.Value(false), // debug flag
		NewDefaultPromptRunner,

		// Session and context management
		ProvideSessionManager,
		ProvideContextManager,

		// Event bus
		ProvideEventBus,

		// Genie factory function
		newGenieCore,
	)
	return nil, nil
}
