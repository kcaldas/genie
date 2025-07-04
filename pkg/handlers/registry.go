package handlers

import (
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// NewDefaultHandlerRegistry creates a registry with default handlers
func NewDefaultHandlerRegistry(eventBus events.EventBus) ai.ResponseHandlerRegistry {
	registry := ai.NewHandlerRegistry()

	// Register default handlers
	handlers := []ai.ResponseHandler{
		NewFileGenerationHandler(eventBus, eventBus), // eventBus implements both EventBus and Publisher
	}

	for _, handler := range handlers {
		// Safe to ignore error since we control these handlers
		_ = registry.Register(handler)
	}

	return registry
}
