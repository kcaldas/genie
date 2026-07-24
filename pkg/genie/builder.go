package genie

import (
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/persona"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/kcaldas/genie/pkg/tools"
)

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

// ProvideGenie provides a complete Genie instance with default options and a
// per-instance event bus. It delegates to ProvideGenieWithOptions so there is
// exactly one description of the object graph.
//
// It lives in this untagged file (not wire_gen.go) so it exists under BOTH
// build-tag configurations: consumers like cmd/bootstrap must compile inside
// Wire's own wireinject-tagged package load, where wire_gen.go is excluded.
func ProvideGenie() (Genie, error) {
	return ProvideGenieWithOptions(applyOptions())
}

// NewGenieWithComponents assembles a Genie from explicitly provided
// components. Most callers should prefer NewGenie with options; this
// constructor exists for advanced embedders and test harnesses (see
// pkg/genie/genietest) that need full control over every dependency.
// The recorder may be nil (session recording disabled).
func NewGenieWithComponents(
	promptRunner PromptRunner,
	sessionMgr SessionManager,
	contextMgr ctx.ContextManager,
	eventBus events.EventBus,
	outputFormatter tools.OutputFormatter,
	personaManager persona.PersonaManager,
	configMgr config.Manager,
	toolRegistry tools.Registry,
	recorder *session.Recorder,
) Genie {
	return newGenieCore(
		promptRunner,
		sessionMgr,
		contextMgr,
		eventBus,
		outputFormatter,
		personaManager,
		configMgr,
		toolRegistry,
		recorder,
	)
}
