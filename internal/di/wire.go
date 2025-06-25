//go:build wireinject

package di

import (
	"github.com/google/wire"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/handlers"
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

// ProvideChainFactory provides the chain factory based on environment configuration
func ProvideChainFactory() (genie.ChainFactory, error) {
	eventBus := ProvideEventBus()
	promptLoader, err := ProvidePromptLoader()
	if err != nil {
		return nil, err
	}
	
	// Check environment variable to choose chain factory
	// GENIE_CHAIN_FACTORY=generalist to use the simpler single-prompt approach
	// Default: use the multi-step clarification-based approach
	chainFactoryType := config.NewConfigManager().GetStringWithDefault("GENIE_CHAIN_FACTORY", "default")
	
	switch chainFactoryType {
	case "generalist":
		return genie.NewGeneralistChainFactory(eventBus, promptLoader), nil
	default:
		return genie.NewDefaultChainFactory(eventBus, promptLoader), nil
	}
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
		ProvideContextManager,

		// Event bus dependency
		ProvideEventBus,

		// Tool output formatter dependency
		ProvideOutputFormatter,

		// Handler registry dependency
		ProvideHandlerRegistry,

		// Chain factory dependency
		ProvideChainFactory,

		// Configuration dependency
		ProvideConfigManager,

		// Genie factory function
		genie.NewGenie,
	)
	return nil, nil
}
