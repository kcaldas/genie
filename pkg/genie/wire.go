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

// Shared skill manager instance (lazy initialized)
var (
	skillManager     skills.SkillManager
	skillManagerOnce sync.Once
	skillManagerErr  error
)

// --- Event bus providers ---
// Each Genie instance gets its own event bus.
// Wire shares the result within a single injection graph,
// so all components of one Genie instance share the same bus.

// provideNewEventBus creates a fresh event bus for each Genie instance.
func provideNewEventBus() events.EventBus {
	return events.NewEventBus()
}

// providePublisher adapts EventBus to Publisher interface for Wire.
func providePublisher(eb events.EventBus) events.Publisher {
	return eb
}

// provideSubscriber adapts EventBus to Subscriber interface for Wire.
func provideSubscriber(eb events.EventBus) events.Subscriber {
	return eb
}

// --- Shared providers (stateless or lazy singletons) ---

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
	return mcp.NewLazyMCPClient(), nil
}

// ProvideConfigManager provides a configuration manager
func ProvideConfigManager() config.Manager {
	return config.NewConfigManager()
}

// --- Tool registry providers ---

// newRegistryWithOptions is a provider function that creates a registry based on options
func newRegistryWithOptions(
	eventBus events.EventBus,
	todoManager tools.TodoManager,
	skillManager tools.SkillManager,
	mcpClient tools.MCPClient,
	options *GenieOptions,
) (tools.Registry, error) {
	if options.CustomRegistry != nil {
		return options.CustomRegistry, nil
	}

	if options.CustomRegistryFactory != nil {
		return options.CustomRegistryFactory(eventBus, todoManager), nil
	}

	registry := tools.NewDefaultRegistry(eventBus, todoManager, skillManager, mcpClient)

	for _, tool := range options.CustomTools {
		if err := registry.Register(tool); err != nil {
			return nil, fmt.Errorf("failed to register custom tool: %w (hint: check .mcp.json for conflicting tool names)", err)
		}
	}

	return registry, nil
}

// --- AI Gen provider ---

// provideAIGen creates the AI Gen with the given event bus (per-instance).
func provideAIGen(eb events.EventBus, configManager config.Manager) (ai.Gen, error) {
	provider := strings.ToLower(configManager.GetStringWithDefault("GENIE_LLM_PROVIDER", "genai"))

	factories := map[string]multiplexer.Factory{
		"genai":     func() (ai.Gen, error) { return genai.NewClient(eb) },
		"openai":    func() (ai.Gen, error) { return openai.NewClient(eb) },
		"anthropic": func() (ai.Gen, error) { return anthropic.NewClient(eb) },
		"ollama":    func() (ai.Gen, error) { return ollama.NewClient(eb) },
		"lmstudio":  func() (ai.Gen, error) { return lmstudio.NewClient(eb) },
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

	captureConfig := ai.GetCaptureConfigFromEnv(captureProvider)
	if captureConfig.Enabled {
		baseGen = ai.NewCaptureMiddleware(baseGen, captureConfig)
	}

	retryConfig := ai.GetRetryConfigFromEnv(configManager)
	if retryConfig.Enabled {
		return ai.NewRetryMiddleware(baseGen, retryConfig), nil
	}

	return baseGen, nil
}

// --- Context registry provider ---

// provideContextRegistry creates the context registry using the given event bus.
func provideContextRegistry(
	eb events.EventBus,
	skillManager skills.SkillManager,
) *ctx.ContextPartProviderRegistry {
	registry := ctx.NewContextPartProviderRegistry()

	projectManager := ctx.NewProjectCtxManager(eb)
	chatManager := ctx.NewChatCtxManager(eb)
	fileProvider := ctx.NewFileContextPartsProvider(eb)
	todoProvider := ctx.NewTodoContextPartProvider(eb)
	skillProvider := skills.NewSkillContextPartProvider(skillManager, eb)

	chatManager.SetBudgetStrategy(ctx.NewSlidingWindowStrategy())
	fileProvider.SetCollectionStrategy(ctx.NewLRUStrategy(10))
	fileProvider.SetContentStrategy(ctx.NewSoftTrimStrategy(1500, 1500))

	registry.Register(projectManager, 0)
	registry.Register(chatManager, 0.7)
	registry.Register(fileProvider, 0.3)
	registry.Register(todoProvider, 0)

	if skillProvider != nil {
		registry.Register(skillProvider, 0)
	}

	return registry
}

// --- Main Genie injectors ---
// These flatten all dependencies into a single wire.Build so that
// provideNewEventBus is called ONCE and shared across all components.

// ProvideGenie provides a complete Genie instance with a per-instance event bus.
func ProvideGenie() (Genie, error) {
	wire.Build(
		// Per-instance event bus (shared within this injection graph)
		provideNewEventBus,
		providePublisher,

		// AI Gen + prompt runner
		provideAIGen,
		ProvideConfigManager,
		wire.Value(false), // debug flag
		NewDefaultPromptRunner,

		// Session manager
		NewSessionManager,

		// Context manager
		ProvideSkillManager,
		provideContextRegistry,
		ctx.NewContextManager,

		// Tool registry
		ProvideTodoManager,
		ProvideMCPClient,
		tools.NewDefaultRegistry,
		tools.NewOutputFormatter,

		// Prompt loader + persona
		prompts.NewPromptLoader,
		persona.NewPersonaPromptFactory,
		persona.NewDefaultPersonaManager,

		// Core
		newGenieCore,
	)
	return nil, nil
}

// ProvideGenieWithOptions provides a complete Genie instance with custom options
// and a per-instance event bus.
func ProvideGenieWithOptions(options *GenieOptions) (Genie, error) {
	wire.Build(
		// Per-instance event bus (shared within this injection graph)
		provideNewEventBus,
		providePublisher,

		// AI Gen + prompt runner
		provideAIGen,
		ProvideConfigManager,
		wire.Value(false), // debug flag
		NewDefaultPromptRunner,

		// Session manager
		NewSessionManager,

		// Context manager
		ProvideSkillManager,
		provideContextRegistry,
		ctx.NewContextManager,

		// Tool registry (with options)
		ProvideTodoManager,
		ProvideMCPClient,
		newRegistryWithOptions,
		tools.NewOutputFormatter,

		// Prompt loader + persona
		prompts.NewPromptLoader,
		persona.NewPersonaPromptFactory,
		persona.NewDefaultPersonaManager,

		// Core
		newGenieCore,
	)
	return nil, nil
}

// --- Standalone providers (for tests, CLI, etc.) ---
// These create their own event bus since they're called independently.

// ProvideToolRegistry provides a tool registry (standalone, own event bus).
func ProvideToolRegistry() (tools.Registry, error) {
	wire.Build(provideNewEventBus, ProvideTodoManager, ProvideSkillManager, ProvideMCPClient, tools.NewDefaultRegistry)
	return nil, nil
}

// ProvideOutputFormatter provides a tool output formatter (standalone).
func ProvideOutputFormatter() (tools.OutputFormatter, error) {
	wire.Build(ProvideToolRegistry, tools.NewOutputFormatter)
	return nil, nil
}

// ProvidePromptLoader provides a prompt loader (standalone, own event bus).
func ProvidePromptLoader() (prompts.Loader, error) {
	wire.Build(provideNewEventBus, providePublisher, ProvideToolRegistry, prompts.NewPromptLoader)
	return nil, nil
}

// ProvideGen provides an AI Gen (standalone, own event bus).
func ProvideGen() (ai.Gen, error) {
	wire.Build(provideNewEventBus, provideAIGen, ProvideConfigManager)
	return nil, nil
}

// ProvidePromptRunner provides a prompt runner (standalone).
func ProvidePromptRunner() (PromptRunner, error) {
	wire.Build(ProvideGen, wire.Value(false), NewDefaultPromptRunner)
	return nil, nil
}

// ProvideSessionManager provides a session manager (standalone, own event bus).
func ProvideSessionManager() SessionManager {
	wire.Build(provideNewEventBus, providePublisher, NewSessionManager)
	return nil
}

// ProvideContextManager provides a context manager (standalone, own event bus).
func ProvideContextManager() (ctx.ContextManager, error) {
	wire.Build(provideNewEventBus, ProvideSkillManager, provideContextRegistry, ctx.NewContextManager)
	return nil, nil
}

// ProvidePersonaManager provides a persona manager (standalone).
func ProvidePersonaManager() (persona.PersonaManager, error) {
	wire.Build(
		provideNewEventBus,
		providePublisher,
		ProvideToolRegistry,
		prompts.NewPromptLoader,
		ProvideSkillManager,
		persona.NewPersonaPromptFactory,
		ProvideConfigManager,
		persona.NewDefaultPersonaManager,
	)
	return nil, nil
}
