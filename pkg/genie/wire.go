//go:build wireinject

package genie

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/wire"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/anthropic"
	"github.com/kcaldas/genie/pkg/llm/genai"
	"github.com/kcaldas/genie/pkg/llm/lmstudio"
	"github.com/kcaldas/genie/pkg/llm/multiplexer"
	"github.com/kcaldas/genie/pkg/llm/ollama"
	"github.com/kcaldas/genie/pkg/llm/openai"
	"github.com/kcaldas/genie/pkg/mcp"
	"github.com/kcaldas/genie/pkg/persona"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/skills"
	"github.com/kcaldas/genie/pkg/tools"
)

// Shared event bus instance
var eventBus = events.NewEventBus()

// Shared skill manager instance (lazy initialized)
var (
	skillManager     skills.SkillManager
	skillManagerOnce sync.Once
	skillManagerErr  error
)

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

// ProvideSkillManager provides a shared skill manager instance
func ProvideSkillManager() (skills.SkillManager, error) {
	skillManagerOnce.Do(func() {
		skillManager, skillManagerErr = skills.NewDefaultSkillManager()
	})
	return skillManager, skillManagerErr
}

// ProvideMCPClient provides a lazy MCP client (uninitialized until registry.Init is called)
func ProvideMCPClient() (tools.MCPClient, error) {
	// Return a lazy client that will be initialized with the working directory
	// when registry.Init() is called in Genie.Start()
	return mcp.NewLazyMCPClient(), nil
}

// ProvideToolRegistry provides a tool registry with interactive tools and MCP tools
func ProvideToolRegistry() (tools.Registry, error) {
	wire.Build(ProvideEventBus, ProvideTodoManager, ProvideSkillManager, ProvideMCPClient, tools.NewRegistryWithMCP)
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
		ProvideSkillManager,
		ProvideMCPClient,
		newRegistryWithOptions,
	)
	return nil, nil
}

// newRegistryWithOptions is a provider function that creates a registry based on options
func newRegistryWithOptions(
	eventBus events.EventBus,
	todoManager tools.TodoManager,
	skillManager tools.SkillManager,
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
	registry := tools.NewRegistryWithMCP(eventBus, todoManager, skillManager, mcpClient)

	// Add any custom tools to the default registry
	for _, tool := range options.CustomTools {
		if err := registry.Register(tool); err != nil {
			return nil, fmt.Errorf("failed to register custom tool: %w (hint: check .mcp.json for conflicting tool names)", err)
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

func ProvideSkillContextPartProvider() (*skills.SkillContextPartProvider, error) {
	wire.Build(ProvideSkillManager, ProvideEventBus, skills.NewSkillContextPartProvider)
	return nil, nil
}

func ProvideContextRegistry() *ctx.ContextPartProviderRegistry {
	// Create registry
	registry := ctx.NewContextPartProviderRegistry()

	// Get managers (they now implement ContextPartProvider directly)
	projectManager := ProvideProjectCtxManager()
	chatManager := ProvideChatCtxManager()
	fileProvider := ProvideFileContextPartsProvider()
	todoProvider := ProvideTodoContextPartsProvider()
	skillProvider, _ := ProvideSkillContextPartProvider()

	// Wire default budget strategies
	chatManager.SetBudgetStrategy(ctx.NewSlidingWindowStrategy())
	fileProvider.SetCollectionStrategy(ctx.NewLRUStrategy(10))
	fileProvider.SetContentStrategy(ctx.NewSoftTrimStrategy(1500, 1500))

	// Register providers with budget shares (chat and files share the budget)
	registry.Register(projectManager, 0)
	registry.Register(chatManager, 0.7)
	registry.Register(fileProvider, 0.3)
	registry.Register(todoProvider, 0)

	if skillProvider != nil {
		registry.Register(skillProvider, 0)
	}

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
		"lmstudio":  func() (ai.Gen, error) { return lmstudio.NewClient(eventBus) },
	}

	aliases := map[string]string{
		"gemini":           "genai",
		"google":           "genai",
		"vertex":           "genai",
		"openai-chat":      "openai",
		"claude":           "anthropic",
		"anthropic-claude": "anthropic",
		"lm-studio":        "lmstudio",
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
	wire.Build(
		ProvidePromptLoader,
		ProvideSkillManager,
		persona.NewPersonaPromptFactory,
	)
	return nil, nil
}

// ProviderPromptRunner provides the prompt runner
func ProvidePromptRunner() (PromptRunner, error) {
	wire.Build(ProvideGen, wire.Value(false), NewDefaultPromptRunner)
	return nil, nil
}

// ProvidePersonaManager provides the persona manager
func ProvidePersonaManager() (persona.PersonaManager, error) {
	wire.Build(ProvidePersonaPromptFactory, ProvideConfigManager, ProvidePublisher, persona.NewDefaultPersonaManager)
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

		// Tool registry dependency
		ProvideToolRegistry,

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
		ProvideSkillManager,
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

		// Genie factory function (registry is already provided via ProvideToolRegistryWithOptions)
		newGenieCore,
	)
	return nil, nil
}
