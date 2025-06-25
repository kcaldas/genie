package ctx

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/events"
)

// ContextManager manages conversation context for sessions
type ContextManager interface {
	GetLLMContext(ctx context.Context, sessionID string) (string, error) // Returns formatted context for LLM with internal window size
	ClearContext(sessionID string) error
}

// InMemoryManager implements ContextManager with in-memory storage
type InMemoryManager struct {
	contexts          map[string][]string
	projectCtxManager ProjectCtxManager
}

// NewContextManager creates a new context manager that subscribes to events from a bus
func NewContextManager(subscriber events.Subscriber, projectCtxManager ProjectCtxManager) ContextManager {
	manager := &InMemoryManager{
		contexts:          make(map[string][]string),
		projectCtxManager: projectCtxManager,
	}

	// Subscribe to chat response events
	subscriber.Subscribe("chat.response", manager.handleChatResponseEvent)

	return manager
}

// NewInMemoryContextManager creates a new in-memory context manager without event subscription (for testing)
func NewInMemoryContextManager() ContextManager {
	// Create a minimal project context manager for testing
	projectCtxManager := NewProjectCtxManager(nil)
	return &InMemoryManager{
		contexts:          make(map[string][]string),
		projectCtxManager: projectCtxManager,
	}
}

// handleChatResponseEvent handles chat response events from the event bus
func (m *InMemoryManager) handleChatResponseEvent(event interface{}) {
	if chatEvent, ok := event.(events.ChatResponseEvent); ok {
		// Add the user message and assistant response to context
		m.addInteraction(chatEvent.SessionID, chatEvent.Message, chatEvent.Response)
	}
}


// addInteraction adds a user message and assistant response to the session context
func (m *InMemoryManager) addInteraction(sessionID, userMessage, assistantResponse string) {
	if _, exists := m.contexts[sessionID]; !exists {
		m.contexts[sessionID] = make([]string, 0)
	}

	m.contexts[sessionID] = append(m.contexts[sessionID], userMessage, assistantResponse)
}

// getSessionContext retrieves the conversation context for a session, including project context at the top
func (m *InMemoryManager) getSessionContext(ctx context.Context, sessionID string) ([]string, error) {
	var result []string

	// Add project context at the top
	projectContext, err := m.projectCtxManager.GetContext(ctx)
	if err != nil {
		return nil, err
	}
	if projectContext != "" {
		result = append(result, projectContext)
	}

	// Add session context
	sessionContext, exists := m.contexts[sessionID]
	if exists {
		result = append(result, sessionContext...)
	}

	return result, nil
}

// GetLLMContext returns formatted context for LLM with internal default window size
func (m *InMemoryManager) GetLLMContext(ctx context.Context, sessionID string) (string, error) {
	contextData, err := m.getSessionContext(ctx, sessionID)
	if err != nil {
		return "", err
	}

	// Use internal default of 5 pairs - this can become configurable later
	const defaultMaxPairs = 5

	// Context comes as alternating user/assistant messages (after project context)
	// Skip project context if present and only include complete pairs
	var sessionMessages []string
	if len(contextData) > 0 {
		// Check if first item is project context (contains markdown-like content)
		if len(contextData) > 0 && strings.Contains(contextData[0], "#") {
			// First item is likely project context, skip it
			sessionMessages = contextData[1:]
		} else {
			sessionMessages = contextData
		}
	}

	totalMessages := len(sessionMessages)
	if totalMessages%2 != 0 {
		totalMessages-- // Remove incomplete pair
	}

	if totalMessages == 0 {
		// Only project context, return it
		if len(contextData) > 0 && strings.Contains(contextData[0], "#") {
			return contextData[0], nil
		}
		return "", nil
	}

	pairsToInclude := defaultMaxPairs
	if totalMessages/2 < defaultMaxPairs {
		pairsToInclude = totalMessages / 2
	}

	startIdx := totalMessages - (pairsToInclude * 2)

	var contextBuilder strings.Builder
	
	// Add project context first if it exists
	if len(contextData) > 0 && strings.Contains(contextData[0], "#") {
		contextBuilder.WriteString(contextData[0])
		contextBuilder.WriteString("\n\n")
	}

	// Add conversation pairs
	for i := startIdx; i < totalMessages; i += 2 {
		contextBuilder.WriteString(fmt.Sprintf("User: %s\n", sessionMessages[i]))
		contextBuilder.WriteString(fmt.Sprintf("Assistant: %s\n", sessionMessages[i+1]))
	}

	return strings.TrimSpace(contextBuilder.String()), nil
}

// ClearContext removes all context for a session
func (m *InMemoryManager) ClearContext(sessionID string) error {
	delete(m.contexts, sessionID)
	return nil
}
