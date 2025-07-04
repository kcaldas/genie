package genie

import (
	"context"
	"fmt"
	"maps"
	"os"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/kcaldas/genie/pkg/tools"
)

// ChainFactory creates conversation chains - allows tests to inject custom chains
type ChainFactory interface {
	CreateChatChain() (*ai.Chain, error)
}

// ChainRunner executes chains - allows mocking chain execution for testing
type ChainRunner interface {
	RunChain(ctx context.Context, chain *ai.Chain, chainCtx *ai.ChainContext, eventBus events.EventBus) error
}

// DefaultChainRunner is the production implementation that runs chains through the LLM
type DefaultChainRunner struct {
	llmClient       ai.Gen
	handlerRegistry ai.HandlerRegistry
	debug           bool
}

// NewDefaultChainRunner creates a new DefaultChainRunner
func NewDefaultChainRunner(llmClient ai.Gen, handlerRegistry ai.HandlerRegistry, debug bool) ChainRunner {
	return &DefaultChainRunner{
		llmClient:       llmClient,
		handlerRegistry: handlerRegistry,
		debug:           debug,
	}
}

// RunChain executes the chain using the real LLM client
func (r *DefaultChainRunner) RunChain(ctx context.Context, chain *ai.Chain, chainCtx *ai.ChainContext, eventBus events.EventBus) error {
	// Inject handler registry into context
	ctx = context.WithValue(ctx, "handlerRegistry", r.handlerRegistry)
	return chain.Run(ctx, r.llmClient, chainCtx, eventBus, r.debug)
}

// core is the main implementation of the Genie interface
type core struct {
	chainRunner     ChainRunner
	sessionMgr      session.SessionManager
	contextMgr      ctx.ContextManager
	eventBus        events.EventBus
	outputFormatter tools.OutputFormatter
	handlerRegistry ai.HandlerRegistry
	chainFactory    ChainFactory
	configMgr       config.Manager
	started         bool
}

// NewGenie creates a new Genie core instance with dependency injection
func NewGenie(
	chainRunner ChainRunner,
	sessionMgr session.SessionManager,
	contextMgr ctx.ContextManager,
	eventBus events.EventBus,
	outputFormatter tools.OutputFormatter,
	handlerRegistry ai.HandlerRegistry,
	chainFactory ChainFactory,
	configMgr config.Manager,
) Genie {
	return &core{
		chainRunner:     chainRunner,
		sessionMgr:      sessionMgr,
		contextMgr:      contextMgr,
		eventBus:        eventBus,
		outputFormatter: outputFormatter,
		handlerRegistry: handlerRegistry,
		chainFactory:    chainFactory,
		configMgr:       configMgr,
	}
}

// Start initializes Genie with working directory and returns initial session
func (g *core) Start(workingDir *string) (*Session, error) {
	if g.started {
		return nil, fmt.Errorf("Genie has already been started")
	}

	// Determine actual working directory
	var actualWorkingDir string
	if workingDir == nil {
		// Default to current directory
		if currentDir, err := os.Getwd(); err == nil {
			actualWorkingDir = currentDir
		} else {
			actualWorkingDir = "." // fallback
		}
	} else {
		actualWorkingDir = *workingDir
	}

	// Validate working directory exists
	if _, err := os.Stat(actualWorkingDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("working directory does not exist: %s", actualWorkingDir)
	}

	// Mark as started
	g.started = true

	// Skip early AI check for fast startup - LLM will be initialized on first chat

	// Create initial session
	_, err := g.sessionMgr.CreateSession(actualWorkingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial session: %w", err)
	}

	// Return session
	return &Session{
		WorkingDirectory: actualWorkingDir,
		CreatedAt:        "TODO", // We'll add timestamps later
		Interactions:     []Interaction{},
	}, nil
}

func (g *core) ensureStarted() error {
	if !g.started {
		return fmt.Errorf("Genie must be started before use - call Start() first")
	}
	return nil
}

// Chat processes a chat message asynchronously and publishes the response via events
func (g *core) Chat(ctx context.Context, message string) error {
	if err := g.ensureStarted(); err != nil {
		return err
	}

	// Publish started event immediately
	startEvent := events.ChatStartedEvent{
		Message: message,
	}
	g.eventBus.Publish(startEvent.Topic(), startEvent)

	// Process chat asynchronously
	go func() {
		// Recover from panics to ensure response event is always published
		defer func() {
			if r := recover(); r != nil {
				panicErr := fmt.Errorf("internal error: %v", r)
				responseEvent := events.ChatResponseEvent{
					Message:  message,
					Response: "",
					Error:    panicErr,
				}
				g.eventBus.Publish(responseEvent.Topic(), responseEvent)
			}
		}()

		response, err := g.processChat(ctx, message)

		// Publish response event (success or error)
		responseEvent := events.ChatResponseEvent{
			Message:  message,
			Response: response,
			Error:    err,
		}
		g.eventBus.Publish(responseEvent.Topic(), responseEvent)
	}()

	return nil
}

// GetSession retrieves an existing session
func (g *core) GetSession() (*Session, error) {
	if err := g.ensureStarted(); err != nil {
		return nil, err
	}

	sess, err := g.sessionMgr.GetSession()
	if err != nil {
		return nil, err
	}

	// Convert internal session to public Session type
	// For now, return a basic session - we'll enhance this later
	return &Session{
		WorkingDirectory: sess.GetWorkingDirectory(),
		CreatedAt:        "TODO", // We'll need to add timestamp to session interface
	}, nil
}

// GetContext returns the same context that would be sent to the LLM
func (g *core) GetContext(ctx context.Context) (map[string]string, error) {
	if err := g.ensureStarted(); err != nil {
		return nil, err
	}

	// Get session to set up context properly
	sess, err := g.sessionMgr.GetSession()
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Add working directory to context for handlers
	ctx = context.WithValue(ctx, "cwd", sess.GetWorkingDirectory())

	// Return structured context parts
	return g.contextMgr.GetContextParts(ctx)
}

// GetEventBus returns the event bus for async communication
func (g *core) GetEventBus() events.EventBus {
	return g.eventBus
}

// Reset resets the started state for testing purposes
func (g *core) Reset() {
	g.started = false
}

// processChat handles the actual chat processing logic
func (g *core) processChat(ctx context.Context, message string) (string, error) {
	// Get session (must exist since Start() creates initial session)
	sess, err := g.sessionMgr.GetSession()
	if err != nil {
		return "", fmt.Errorf("session not found: %w - use session ID from Start() method", err)
	}

	// Build conversation context parts
	contextParts, err := g.contextMgr.GetContextParts(ctx)
	if err != nil {
		// If context retrieval fails, continue with empty context
		contextParts = make(map[string]string)
	}

	// Require ChainFactory to be provided via dependency injection
	if g.chainFactory == nil {
		return "", fmt.Errorf("no ChainFactory provided - chain creation must be explicitly configured")
	}

	chain, err := g.chainFactory.CreateChatChain()
	if err != nil {
		return "", fmt.Errorf("failed to create chain: %w", err)
	}

	// Create chain context with structured context parts + message
	chainData := make(map[string]string)
	maps.Copy(chainData, contextParts)

	// Add the user message
	chainData["message"] = message

	chainCtx := ai.NewChainContext(chainData)

	// working directory to context for handlers
	ctx = context.WithValue(ctx, "cwd", sess.GetWorkingDirectory())

	// Add configurable LLM recursion depth limit
	maxRecursionDepth := g.configMgr.GetIntWithDefault("GENIE_LLM_MAX_RECURSION_DEPTH", 50)
	ctx = context.WithValue(ctx, "maxCalls", maxRecursionDepth)

	err = g.chainRunner.RunChain(ctx, chain, chainCtx, g.eventBus)
	if err != nil {
		return "", fmt.Errorf("failed to execute chat chain: %w", err)
	}

	response := chainCtx.Data["response"]

	// Format tool outputs in the response for better user experience
	formattedResponse := g.outputFormatter.FormatResponse(response)

	return formattedResponse, nil
}
