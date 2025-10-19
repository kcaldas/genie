package genie

import (
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
)

// GenieOptions holds configuration options for creating a Genie instance
type GenieOptions struct {
	// CustomRegistry allows full control over the tool registry
	// If nil, a default registry will be created
	CustomRegistry tools.Registry

	// CustomTools are tools to add to the default registry
	// Ignored if CustomRegistry is set
	CustomTools []tools.Tool

	// CustomRegistryFactory allows advanced customization of the registry
	// Called with the default registry (eventBus, todoManager) dependencies
	// Ignored if CustomRegistry is set
	CustomRegistryFactory func(eventBus events.EventBus, todoManager tools.TodoManager) tools.Registry
}

// GenieOption is a function that configures GenieOptions
type GenieOption func(*GenieOptions)

// WithCustomTools adds custom tools to the default registry
// This is the simplest way to extend Genie with your own tools
//
// Example:
//
//	myTool := NewMyCustomTool()
//	genie, err := genie.NewGenie(genie.WithCustomTools(myTool))
func WithCustomTools(customTools ...tools.Tool) GenieOption {
	return func(opts *GenieOptions) {
		opts.CustomTools = append(opts.CustomTools, customTools...)
	}
}

// WithToolRegistry provides full control over the tool registry
// Use this when you want to completely replace the default tools
//
// Example:
//
//	registry := tools.NewRegistry()
//	registry.Register(NewMyTool1())
//	registry.Register(NewMyTool2())
//	genie, err := genie.NewGenie(genie.WithToolRegistry(registry))
func WithToolRegistry(registry tools.Registry) GenieOption {
	return func(opts *GenieOptions) {
		opts.CustomRegistry = registry
	}
}

// WithCustomRegistryFactory allows advanced customization of the registry
// The factory receives the default dependencies (eventBus, todoManager)
// and can use them to create a customized registry
//
// Example:
//
//	genie, err := genie.NewGenie(genie.WithCustomRegistryFactory(
//	    func(eventBus events.EventBus, todoManager tools.TodoManager) tools.Registry {
//	        registry := tools.NewDefaultRegistry(eventBus, todoManager)
//	        registry.Register(NewMyTool())
//	        return registry
//	    },
//	))
func WithCustomRegistryFactory(factory func(events.EventBus, tools.TodoManager) tools.Registry) GenieOption {
	return func(opts *GenieOptions) {
		opts.CustomRegistryFactory = factory
	}
}

// applyOptions applies all options to create a final GenieOptions
func applyOptions(opts ...GenieOption) *GenieOptions {
	options := &GenieOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}
