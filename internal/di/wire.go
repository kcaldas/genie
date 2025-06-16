//go:build wireinject

package di

import (
	"github.com/google/wire"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/kcaldas/genie/pkg/llm/vertex"
	"github.com/kcaldas/genie/pkg/session"
)

// Wire provider functions
func ProvideContextManager() context.ContextManager {
	return context.NewContextManager()
}

func ProvideHistoryManager() history.HistoryManager {
	return history.NewHistoryManager()
}

func ProvideSessionManager(contextManager context.ContextManager, historyManager history.HistoryManager) session.SessionManager {
	return session.NewSessionManagerWithManagers(contextManager, historyManager)
}

// InitializeGen is an injector function - Wire will generate the implementation
func InitializeGen() (ai.Gen, error) {
	wire.Build(vertex.NewClientWithError)
	return nil, nil
}

// InitializeSessionManager is an injector function - Wire will generate the implementation
func InitializeSessionManager() (session.SessionManager, error) {
	wire.Build(ProvideContextManager, ProvideHistoryManager, ProvideSessionManager)
	return nil, nil
}