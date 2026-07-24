package genie

import (
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/session"
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

	// TaskExecutor overrides Genie's default subprocess executor for the Task tool.
	TaskExecutor tools.TaskExecutor

	// TaskCompletionHandler observes terminal async Task results.
	TaskCompletionHandler tools.TaskCompletionHandler

	// SessionRecorder is a host-owned session recorder. Hosts that need
	// to append their own entries (session.Recorder.AppendCustom) build
	// the recorder themselves and hand it over here.
	// Takes precedence over SessionStorage/SessionRecordingLevel.
	SessionRecorder *session.Recorder

	// SessionStorage is where session recording writes when no
	// host-owned recorder is provided. Nil disables recording.
	SessionStorage session.Storage

	// SessionRecordingLevel controls how much the session recorder
	// captures. LevelOff (the zero value) disables recording.
	SessionRecordingLevel session.Level
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
//	        registry := tools.NewDefaultRegistry(eventBus, todoManager, nil, nil)
//	        registry.Register(NewMyTool())
//	        return registry
//	    },
//	))
func WithCustomRegistryFactory(factory func(events.EventBus, tools.TodoManager) tools.Registry) GenieOption {
	return func(opts *GenieOptions) {
		opts.CustomRegistryFactory = factory
	}
}

// WithTaskExecutor configures how the built-in Task tool runs work.
func WithTaskExecutor(executor tools.TaskExecutor) GenieOption {
	return func(opts *GenieOptions) {
		opts.TaskExecutor = executor
	}
}

// WithTaskCompletionHandler configures a callback for completed, failed,
// cancelled, or timed-out Task invocations.
func WithTaskCompletionHandler(handler tools.TaskCompletionHandler) GenieOption {
	return func(opts *GenieOptions) {
		opts.TaskCompletionHandler = handler
	}
}

// WithSessionRecorder attaches a host-owned session recorder. Use this when
// the host needs the recorder handle itself — e.g. to stamp opaque custom
// entries (AppendCustom) around chat calls — without widening the Genie
// interface. The host also owns closing it.
//
// Example:
//
//	storage, _ := session.NewDiskJSONL(path)
//	recorder := session.NewRecorder(storage, session.LevelStandard)
//	g, err := genie.NewGenie(genie.WithSessionRecorder(recorder))
func WithSessionRecorder(recorder *session.Recorder) GenieOption {
	return func(opts *GenieOptions) {
		opts.SessionRecorder = recorder
	}
}

// WithSessionRecording enables session recording to the given storage at
// the given level. A nil storage or LevelOff leaves recording disabled.
//
// Example:
//
//	storage, _ := session.NewDiskJSONL(path)
//	g, err := genie.NewGenie(genie.WithSessionRecording(storage, session.LevelStandard))
func WithSessionRecording(storage session.Storage, level session.Level) GenieOption {
	return func(opts *GenieOptions) {
		opts.SessionStorage = storage
		opts.SessionRecordingLevel = level
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
