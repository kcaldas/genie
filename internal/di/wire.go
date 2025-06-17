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

func ProvideContextManager(contextCh chan events.SessionInteractionEvent) context.ContextManager {
	return context.NewContextManager(contextCh)
}

func ProvideHistoryManager(historyCh chan events.SessionInteractionEvent) history.HistoryManager {
	return history.NewHistoryManager(historyCh)
}

func ProvideSessionManager(historyCh, contextCh chan events.SessionInteractionEvent) session.SessionManager {
	return session.NewSessionManager(historyCh, contextCh)
}

// InitializeGen is an injector function - Wire will generate the implementation
func InitializeGen() (ai.Gen, error) {
	wire.Build(vertex.NewClientWithError)
	return nil, nil
}
