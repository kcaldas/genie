package genie

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/ai"
	contextpkg "github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/kcaldas/genie/pkg/tools"
)

// Dependencies contains all the dependencies needed by Genie core
type Dependencies struct {
	LLMClient       ai.Gen
	PromptLoader    prompts.Loader
	SessionMgr      session.SessionManager
	HistoryMgr      history.HistoryManager
	ContextMgr      contextpkg.ContextManager
	ChatHistoryMgr  history.ChatHistoryManager
	EventBus        events.EventBus
	OutputFormatter tools.OutputFormatter
}

// core is the main implementation of the Genie interface
type core struct {
	llmClient       ai.Gen
	promptLoader    prompts.Loader
	sessionMgr      session.SessionManager
	historyMgr      history.HistoryManager
	contextMgr      contextpkg.ContextManager
	chatHistoryMgr  history.ChatHistoryManager
	eventBus        events.EventBus
	outputFormatter tools.OutputFormatter
}

// New creates a new Genie core instance with the provided dependencies
func New(deps Dependencies) Genie {
	return &core{
		llmClient:       deps.LLMClient,
		promptLoader:    deps.PromptLoader,
		sessionMgr:      deps.SessionMgr,
		historyMgr:      deps.HistoryMgr,
		contextMgr:      deps.ContextMgr,
		chatHistoryMgr:  deps.ChatHistoryMgr,
		eventBus:        deps.EventBus,
		outputFormatter: deps.OutputFormatter,
	}
}

// Chat processes a chat message asynchronously and publishes the response via events
func (g *core) Chat(ctx context.Context, sessionID string, message string) error {
	// Publish started event immediately
	startEvent := ChatStartedEvent{
		SessionID: sessionID,
		Message:   message,
	}
	g.eventBus.Publish(startEvent.Topic(), startEvent)
	
	// Process chat asynchronously
	go func() {
		response, err := g.processChat(ctx, sessionID, message)
		
		// Publish response event (success or error)
		responseEvent := ChatResponseEvent{
			SessionID: sessionID,
			Message:   message,
			Response:  response,
			Error:     err,
		}
		g.eventBus.Publish(responseEvent.Topic(), responseEvent)
	}()
	
	return nil
}

// CreateSession creates a new conversation session with generated ID
func (g *core) CreateSession() (string, error) {
	sessionID := uuid.New().String()
	_, err := g.sessionMgr.CreateSession(sessionID)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

// GetSession retrieves an existing session
func (g *core) GetSession(sessionID string) (*Session, error) {
	sess, err := g.sessionMgr.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	
	// Convert internal session to public Session type
	// For now, return a basic session - we'll enhance this later
	return &Session{
		ID:           sess.GetID(),
		CreatedAt:    "TODO", // We'll need to add timestamp to session interface
		Interactions: []Interaction{}, // We'll populate this from session data
	}, nil
}


// processChat handles the actual chat processing logic
func (g *core) processChat(ctx context.Context, sessionID string, message string) (string, error) {
	// Get or create session
	sess, err := g.sessionMgr.GetSession(sessionID)
	if err != nil {
		// Try to create session if it doesn't exist
		sess, err = g.sessionMgr.CreateSession(sessionID)
		if err != nil {
			return "", fmt.Errorf("failed to get or create session: %w", err)
		}
	}
	
	// Build conversation context
	conversationContext, err := g.contextMgr.GetConversationContext(sessionID, 5)
	if err != nil {
		// If context retrieval fails, continue with empty context
		conversationContext = ""
	}
	
	// Load conversation prompt
	prompt, err := g.promptLoader.LoadPrompt("conversation")
	if err != nil {
		return "", fmt.Errorf("failed to load conversation prompt: %w", err)
	}
	
	// Create and execute chain
	chain := &ai.Chain{
		Name: "genie-chat",
		Steps: []interface{}{
			ai.ChainStep{
				Name:      "conversation",
				Prompt:    &prompt,
				ForwardAs: "response",
			},
		},
	}
	
	chainCtx := ai.NewChainContext(map[string]string{
		"context": conversationContext,
		"message": message,
	})
	
	err = chain.Run(ctx, g.llmClient, chainCtx, false)
	if err != nil {
		return "", fmt.Errorf("failed to execute chat chain: %w", err)
	}
	
	response := chainCtx.Data["response"]
	
	// Format tool outputs in the response for better user experience
	formattedResponse := g.outputFormatter.FormatResponse(response)
	
	// Add interaction to session (this will trigger session events via existing event system)
	err = sess.AddInteraction(message, formattedResponse)
	if err != nil {
		// Log error but don't fail the chat - response was generated successfully
		// TODO: Add proper logging here
	}
	
	return formattedResponse, nil
}