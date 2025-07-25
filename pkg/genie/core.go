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
	"github.com/kcaldas/genie/pkg/persona"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/kcaldas/genie/pkg/tools"
)

// PromptRunner executes prompts - allows mocking prompt execution for testing
type PromptRunner interface {
	RunPrompt(ctx context.Context, prompt *ai.Prompt, data map[string]string, eventBus events.EventBus) (string, error)
	CountTokens(ctx context.Context, prompt *ai.Prompt, data map[string]string, eventBus events.EventBus) (*ai.TokenCount, error)
	GetStatus() *ai.Status
}

// DefaultPromptRunner is the production implementation that runs prompts through the LLM
type DefaultPromptRunner struct {
	llmClient ai.Gen
	debug     bool
}

// NewDefaultPromptRunner creates a new DefaultPromptRunner
func NewDefaultPromptRunner(llmClient ai.Gen, debug bool) PromptRunner {
	return &DefaultPromptRunner{
		llmClient: llmClient,
		debug:     debug,
	}
}

func (r *DefaultPromptRunner) RunPrompt(ctx context.Context, prompt *ai.Prompt, data map[string]string, eventBus events.EventBus) (string, error) {
	return r.llmClient.GenerateContentAttr(ctx, *prompt, r.debug, ai.MapToAttr(data))
}

func (r *DefaultPromptRunner) CountTokens(ctx context.Context, prompt *ai.Prompt, data map[string]string, eventBus events.EventBus) (*ai.TokenCount, error) {
	return r.llmClient.CountTokensAttr(ctx, *prompt, r.debug, ai.MapToAttr(data))
}

// GetStatus returns the status from the underlying LLM client
func (r *DefaultPromptRunner) GetStatus() *ai.Status {
	return r.llmClient.GetStatus()
}

// core is the main implementation of the Genie interface
type core struct {
	promptRunner    PromptRunner
	sessionMgr      session.SessionManager
	contextMgr      ctx.ContextManager
	eventBus        events.EventBus
	outputFormatter tools.OutputFormatter
	personaManager  persona.PersonaManager
	configMgr       config.Manager
	started         bool
}

// NewGenie creates a new Genie core instance with dependency injection
func NewGenie(
	promptRunner PromptRunner,
	sessionMgr session.SessionManager,
	contextMgr ctx.ContextManager,
	eventBus events.EventBus,
	outputFormatter tools.OutputFormatter,
	personaManager persona.PersonaManager,
	configMgr config.Manager,
) Genie {
	return &core{
		promptRunner:    promptRunner,
		sessionMgr:      sessionMgr,
		contextMgr:      contextMgr,
		eventBus:        eventBus,
		outputFormatter: outputFormatter,
		personaManager:  personaManager,
		configMgr:       configMgr,
	}
}

// Start initializes Genie with working directory and persona, returns initial session
func (g *core) Start(workingDir *string, persona *string) (*Session, error) {
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

	// Determine actual persona
	var actualPersona string
	if persona != nil {
		actualPersona = *persona
	}

	// Create initial session
	_, err := g.sessionMgr.CreateSession(actualWorkingDir, actualPersona)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial session: %w", err)
	}

	// Return session
	return &Session{
		WorkingDirectory: actualWorkingDir,
		CreatedAt:        "TODO", // We'll add timestamps later
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
	ctx = context.WithValue(ctx, "persona", sess.GetPersona())

	contextMap, err := g.contextMgr.GetContextParts(ctx)
	if err != nil {
		return nil, err
	}

	// Create prompt context with structured context parts + empty message
	promptData := g.preparePromptData(ctx, "")

	// Require PersonaManager to be provided via dependency injection
	if g.personaManager == nil {
		return nil, fmt.Errorf("no PersonaManager provided - prompt creation must be explicitly configured")
	}

	prompt, err := g.personaManager.GetPrompt(ctx)
	if err != nil {
		return nil, err
	}

	tokenCount, err := g.promptRunner.CountTokens(ctx, prompt, promptData, g.eventBus)
	if err != nil {
		return nil, err
	}

	instructions := fmt.Sprintf("Total tokens count (After substitutions): %d\n\nText: %s\n\nInstructions: %s", tokenCount.TotalTokens, prompt.Text, prompt.Instruction)
	contextMap["instructions"] = instructions

	// Return structured context parts
	return contextMap, nil
}

// GetEventBus returns the event bus for async communication
func (g *core) GetEventBus() events.EventBus {
	return g.eventBus
}

// GetStatus returns the current status of the AI backend
func (g *core) GetStatus() *Status {
	aiStatus := g.promptRunner.GetStatus()
	return &Status{
		Connected: aiStatus.Connected,
		Model:     aiStatus.Model,
		Backend:   aiStatus.Backend,
		Message:   aiStatus.Message,
	}
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

	// Add working directory and persona to context BEFORE getting prompt
	ctx = context.WithValue(ctx, "cwd", sess.GetWorkingDirectory())
	ctx = context.WithValue(ctx, "persona", sess.GetPersona())

	// Create prompt context with structured context parts + message
	promptData := g.preparePromptData(ctx, message)

	// Require PersonaManager to be provided via dependency injection
	if g.personaManager == nil {
		return "", fmt.Errorf("no PersonaManager provided - prompt creation must be explicitly configured")
	}

	prompt, err := g.personaManager.GetPrompt(ctx)
	if err != nil {
		return "", err
	}

	response, err := g.promptRunner.RunPrompt(ctx, prompt, promptData, g.eventBus)
	if err != nil {
		return "", fmt.Errorf("failed to execute chat prompt: %w", err)
	}

	// Format tool outputs in the response for better user experience
	formattedResponse := g.outputFormatter.FormatResponse(response)

	return formattedResponse, nil
}

func (g *core) preparePromptData(ctx context.Context, message string) map[string]string {
	// Build conversation context parts
	contextParts, err := g.contextMgr.GetContextParts(ctx)
	if err != nil {
		// If context retrieval fails, continue with empty context
		contextParts = make(map[string]string)
	}

	// Create prompt context with structured context parts + message
	promptData := make(map[string]string)
	maps.Copy(promptData, contextParts)

	// Enhance chat context with todos if they exist
	if todoContent, hasTodo := promptData["todo"]; hasTodo && todoContent != "" {
		if chatContent, hasChat := promptData["chat"]; hasChat {
			// Append todos to the end of chat history
			enhancedChat := chatContent + "\n\n## Current Tasks\n" + todoContent
			promptData["chat"] = enhancedChat
		} else {
			// No chat history, create one with just the todos
			promptData["chat"] = "## Current Tasks\n" + todoContent
		}
		// Remove the separate todo entry since it's now in chat
		delete(promptData, "todo")
	}

	// Add the user message
	promptData["message"] = message

	return promptData
}
