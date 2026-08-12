package genie

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/kcaldas/genie/pkg/tools"
)

// sessionRecorderFromEnv activates session recording without host wiring:
// GENIE_SESSION_RECORDING=standard|full writes JSONL session files under
// <genie home>/.genie/sessions. Files are named by start time (the header
// carries the session id). Invalid values and storage failures warn and
// leave recording off — env activation must never fail startup.
func sessionRecorderFromEnv() *session.Recorder {
	raw := strings.TrimSpace(os.Getenv("GENIE_SESSION_RECORDING"))
	if raw == "" {
		return nil
	}
	level, err := session.ParseLevel(raw)
	if err != nil {
		slog.Warn("session recording disabled: invalid GENIE_SESSION_RECORDING", "value", raw, "error", err)
		return nil
	}
	if level == session.LevelOff {
		return nil
	}
	home, err := os.Getwd()
	if err != nil {
		slog.Warn("session recording disabled: resolve genie home", "error", err)
		return nil
	}
	name := time.Now().UTC().Format("20060102-150405.000000000") + ".session.jsonl"
	path := filepath.Join(home, ".genie", "sessions", name)
	storage, err := session.NewDiskJSONL(path)
	if err != nil {
		slog.Warn("session recording disabled: open session file", "path", path, "error", err)
		return nil
	}
	return session.NewRecorder(storage, level)
}

// GenieOptions holds configuration options for creating a Genie instance
type GenieOptions struct {
	// CapabilityResolver may be shared across many short-lived Genie
	// instances. Hosts such as Mutiro should create one resolver per agent and
	// inject it into every instance to reuse provider model metadata.
	CapabilityResolver *ai.CapabilityResolver

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

// WithCapabilityResolver shares provider model metadata across Genie
// instances. A per-instance in-memory resolver is used when omitted.
func WithCapabilityResolver(resolver *ai.CapabilityResolver) GenieOption {
	return func(opts *GenieOptions) {
		opts.CapabilityResolver = resolver
	}
}

// WithCapabilityStore is the compact host integration when only cache
// persistence is customized. Reuse the same store across instances. Hosts that
// also want cross-instance in-flight coalescing should create and share a
// CapabilityResolver instead.
func WithCapabilityStore(store ai.CapabilityStore) GenieOption {
	return func(opts *GenieOptions) {
		if store != nil {
			opts.CapabilityResolver = ai.NewCapabilityResolver(store)
		}
	}
}

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
