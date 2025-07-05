//go:build wireinject

package di

import (
	"github.com/google/wire"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/handlers"
	"github.com/kcaldas/genie/pkg/llm/genai"
	"github.com/kcaldas/genie/pkg/persona"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/session"
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

// ProvideToolRegistry provides a tool registry with interactive tools
func ProvideToolRegistry() tools.Registry {
	wire.Build(ProvideEventBus, ProvideTodoManager, tools.NewDefaultRegistry)
	return nil
}

// ProvideOutputFormatter provides a tool output formatter
func ProvideOutputFormatter() tools.OutputFormatter {
	wire.Build(ProvideToolRegistry, tools.NewOutputFormatter)
	return nil
}

// ProvideHandlerRegistry provides a handler registry with default handlers
func ProvideReponseHandlerRegistry() ai.ResponseHandlerRegistry {
	wire.Build(ProvideEventBus, handlers.NewDefaultHandlerRegistry)
	return nil
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

func ProvideSessionManager() session.SessionManager {
	wire.Build(ProvidePublisher, session.NewSessionManager)
	return nil
}

// ProvideGen is an injector function - Wire will generate the implementation
func ProvideGen() (ai.Gen, error) {
	wire.Build(ProvideAIGenWithCapture)
	return nil, nil
}

// ProvideAIGenWithCapture creates the AI Gen with optional capture middleware
func ProvideAIGenWithCapture() (ai.Gen, error) {
	// Create the base LLM client using unified GenAI package
	baseGen, err := genai.NewClient()
	if err != nil {
		return nil, err
	}

	// Get capture configuration from environment
	// Use "genai" as the prefix since it supports both backends
	config := ai.GetCaptureConfigFromEnv("genai")

	// Wrap with capture middleware if enabled
	if config.Enabled {
		return ai.NewCaptureMiddleware(baseGen, config), nil
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

// ProvidePersonaChainFactory provides the persona-aware chain factory
func ProvidePersonaChainFactory() (persona.PersonaAwareChainFactory, error) {
	wire.Build(ProvidePromptLoader, persona.NewPersonaChainFactory)
	return nil, nil
}

// ProviderChainRunner provides the chain runner
func ProvideChainRunner() (genie.ChainRunner, error) {
	wire.Build(ProvideGen, ProvideReponseHandlerRegistry, wire.Value(false), genie.NewDefaultChainRunner)
	return nil, nil
}

// ProvidePersonaManager provides the persona manager
func ProvidePersonaManager() (persona.PersonaManager, error) {
	wire.Build(ProvidePersonaChainFactory, persona.NewDefaultPersonaManager)
	return nil, nil
}

// ProvideGenie provides a complete Genie instance using Wire
func ProvideGenie() (genie.Genie, error) {
	wire.Build(
		ProvideChainRunner,

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
		genie.NewGenie,
	)
	return nil, nil
}
