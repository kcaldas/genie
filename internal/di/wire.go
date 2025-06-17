//go:build wireinject

package di

import (
	"github.com/google/wire"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/kcaldas/genie/pkg/llm/vertex"
	"github.com/kcaldas/genie/pkg/session"
)

// Wire provider functions

func ProvideHistoryChannel() chan events.SessionInteractionEvent {
	return make(chan events.SessionInteractionEvent, 10)
}

func ProvideContextChannel() chan events.SessionInteractionEvent {
	return make(chan events.SessionInteractionEvent, 10)
}

func ProvideContextManager() context.ContextManager {
	return context.NewContextManager(ProvideContextChannel())
}

func ProvideHistoryManager() history.HistoryManager {
	return history.NewHistoryManager(ProvideHistoryChannel())
}

func ProvideSessionManager() session.SessionManager {
	return session.NewSessionManager(ProvideHistoryChannel(), ProvideContextChannel())
}

// InitializeGen is an injector function - Wire will generate the implementation
func InitializeGen() (ai.Gen, error) {
	wire.Build(vertex.NewClientWithError)
	return nil, nil
}
