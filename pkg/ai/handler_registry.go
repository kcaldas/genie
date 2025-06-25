package ai

import (
	"context"
	"fmt"
	"sync"
)

// DefaultHandlerRegistry is a thread-safe implementation of HandlerRegistry
type DefaultHandlerRegistry struct {
	handlers map[string]ResponseHandler
	mutex    sync.RWMutex
}

// NewHandlerRegistry creates a new empty handler registry
func NewHandlerRegistry() HandlerRegistry {
	return &DefaultHandlerRegistry{
		handlers: make(map[string]ResponseHandler),
	}
}

// Register adds a handler to the registry
func (r *DefaultHandlerRegistry) Register(handler ResponseHandler) error {
	if handler == nil {
		return fmt.Errorf("cannot register nil handler")
	}
	
	name := handler.Name()
	if name == "" {
		return fmt.Errorf("handler name cannot be empty")
	}
	
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	// Check for duplicate registration
	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("handler with name '%s' already registered", name)
	}
	
	r.handlers[name] = handler
	return nil
}

// GetHandler retrieves a handler by name
func (r *DefaultHandlerRegistry) GetHandler(name string) (ResponseHandler, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	handler, exists := r.handlers[name]
	return handler, exists
}

// ProcessResponse processes a response using the specified handler
func (r *DefaultHandlerRegistry) ProcessResponse(ctx context.Context, handlerName string, response string) (string, error) {
	handler, exists := r.GetHandler(handlerName)
	if !exists {
		return "", fmt.Errorf("no handler found with name '%s'", handlerName)
	}
	
	// If handler can't handle this response, just pass it through unchanged
	if !handler.CanHandle(response) {
		return response, nil
	}
	
	return handler.Process(ctx, response)
}