package genie

// NewGenie creates a new Genie instance with optional customization.
// This is the recommended way to create a Genie instance for external users.
//
// Example - Add custom tools to defaults:
//
//	myTool := NewMyCustomTool()
//	g, err := genie.NewGenie(genie.WithCustomTools(myTool))
//
// Example - Full control over tools:
//
//	registry := tools.NewRegistry()
//	registry.Register(NewMyTool())
//	g, err := genie.NewGenie(genie.WithToolRegistry(registry))
//
// Example - Advanced customization:
//
//	g, err := genie.NewGenie(
//	    genie.WithCustomRegistryFactory(func(eventBus, todoMgr) tools.Registry {
//	        registry := tools.NewDefaultRegistry(eventBus, todoMgr, nil, nil)
//	        registry.Register(NewMyTool())
//	        return registry
//	    }),
//	)
func NewGenie(opts ...GenieOption) (Genie, error) {
	options := applyOptions(opts...)
	return ProvideGenieWithOptions(options)
}

// NewGenieWithDefaults creates a new Genie instance with all default settings.
// This is equivalent to calling NewGenie() with no options.
//
// Example:
//
//	g, err := genie.NewGenieWithDefaults()
func NewGenieWithDefaults() (Genie, error) {
	return NewGenie()
}
