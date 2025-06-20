//go:build wireinject

package di

import (
	"github.com/google/wire"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/kcaldas/genie/pkg/llm/vertex"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/session"
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

// InitializeGen is an injector function - Wire will generate the implementation
func InitializeGen() (ai.Gen, error) {
	wire.Build(vertex.NewClientWithError)
	return nil, nil
}

// InitializePromptLoader is an injector function - Wire will generate the implementation
func InitializePromptLoader() (prompts.Loader, error) {
	wire.Build(ProvidePublisher, prompts.NewPromptLoader)
	return nil, nil
}
