package ai

import (
	"context"
)

// ResponseHandler processes LLM responses to perform actions
type ResponseHandler interface {
	Name() string
	CanHandle(response string) bool
	Process(ctx context.Context, response string) (string, error)
}

// ResponseHandlerRegistry manages response handlers
type ResponseHandlerRegistry interface {
	Register(handler ResponseHandler) error
	GetHandler(name string) (ResponseHandler, bool)
	ProcessResponse(ctx context.Context, handlerName string, response string) (string, error)
}

