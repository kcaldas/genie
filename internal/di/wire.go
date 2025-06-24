//go:build wireinject

package di

import (
	"github.com/google/wire"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/handlers"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/kcaldas/genie/pkg/llm/genai"
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

// ProvideToolRegistry provides a tool registry with interactive tools
func ProvideToolRegistry() tools.Registry {
	wire.Build(ProvideEventBus, tools.NewDefaultRegistry)
	return nil
}

// ProvideOutputFormatter provides a tool output formatter
func ProvideOutputFormatter() tools.OutputFormatter {
	wire.Build(ProvideToolRegistry, tools.NewOutputFormatter)
	return nil
}

// ProvideHandlerRegistry provides a handler registry with default handlers
func ProvideHandlerRegistry() ai.HandlerRegistry {
	wire.Build(ProvideEventBus, handlers.NewDefaultHandlerRegistry)
	return nil
}

// Wire injectors for singleton managers

func ProvideContextManager() context.ContextManager {
	wire.Build(ProvideSubscriber, context.NewContextManager)
	return nil
}

func ProvideHistoryManager() history.HistoryManager {
	wire.Build(ProvideSubscriber, history.NewHistoryManager)
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

// ProvideHistoryPath provides the file path for chat history storage
func ProvideHistoryPath() string {
	return ".genie/history"
}

// ProvideChatHistoryManager provides a chat history manager using Wire
func ProvideChatHistoryManager() history.ChatHistoryManager {
	wire.Build(ProvideHistoryPath, history.NewChatHistoryManager)
	return nil
}

// ProvideChainFactory provides the default chain factory for production
func ProvideChainFactory() (genie.ChainFactory, error) {
	wire.Build(ProvideEventBus, ProvidePromptLoader, genie.NewDefaultChainFactory)
	return nil, nil
}

// ProvideAIProvider provides the production AI provider
func ProvideAIProvider() (genie.AIProvider, error) {
	wire.Build(ProvideGen, ProvideHandlerRegistry, wire.Value(false), genie.NewProductionAIProvider)
	return nil, nil
}


// ProvideGenie provides a complete Genie instance using Wire
func ProvideGenie() (genie.Genie, error) {
	wire.Build(
		// AI provider dependency
		ProvideAIProvider,

		// Prompt dependency
		ProvidePromptLoader,

		// Manager dependencies
		ProvideSessionManager,
		ProvideHistoryManager,
		ProvideContextManager,
		ProvideChatHistoryManager,

		// Event bus dependency
		ProvideEventBus,

		// Tool output formatter dependency
		ProvideOutputFormatter,

		// Handler registry dependency
		ProvideHandlerRegistry,

		// Chain factory dependency
		ProvideChainFactory,

		// Genie factory function
		genie.NewGenie,
	)
	return nil, nil
}
