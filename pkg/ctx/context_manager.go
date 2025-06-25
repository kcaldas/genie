package ctx

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/events"
)

// ContextManager manages conversation context for sessions
type ContextManager interface {
	AddInteraction(sessionID, userMessage, assistantResponse string) error
	GetContext(ctx context.Context, sessionID string) ([]string, error)
	GetConversationContext(ctx context.Context, sessionID string, maxPairs int) (string, error)
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

	// Subscribe to session interaction events
	subscriber.Subscribe("session.interaction", manager.handleEvent)
	
	// Subscribe to tool execution events
	subscriber.Subscribe("tool.executed", manager.handleToolEvent)

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

// handleEvent handles session interaction events from the event bus
func (m *InMemoryManager) handleEvent(event interface{}) {
	if sessionEvent, ok := event.(events.SessionInteractionEvent); ok {
		// Use the existing AddInteraction method
		m.AddInteraction(sessionEvent.SessionID, sessionEvent.UserMessage, sessionEvent.AssistantResponse)
	}
}

// handleToolEvent handles tool execution events from the event bus
func (m *InMemoryManager) handleToolEvent(event interface{}) {
	if toolEvent, ok := event.(events.ToolExecutedEvent); ok {
		// Format tool execution for context
		toolEntry := m.formatToolExecution(toolEvent)
		// Add as a "system" entry - empty user message, tool execution as assistant response
		m.AddInteraction(toolEvent.SessionID, "", toolEntry)
	}
}

// formatToolExecution formats a tool execution event into a readable context entry
func (m *InMemoryManager) formatToolExecution(toolEvent events.ToolExecutedEvent) string {
	// Format parameters as key=value pairs
	var paramPairs []string
	for key, value := range toolEvent.Parameters {
		paramPairs = append(paramPairs, fmt.Sprintf("%s=\"%v\"", key, value))
	}
	
	paramsStr := strings.Join(paramPairs, ", ")
	
	// Format the result content
	var resultStr string
	if toolEvent.Result != nil {
		if success, ok := toolEvent.Result["success"].(bool); ok && success {
			// Extract the main content field from the result
			if results, ok := toolEvent.Result["results"].(string); ok && results != "" {
				resultStr = results
			} else {
				resultStr = "Success"
			}
		} else {
			resultStr = toolEvent.Message
		}
	} else {
		resultStr = toolEvent.Message
	}
	
	return fmt.Sprintf("Tool: %s(%s)\n%s", toolEvent.ToolName, paramsStr, resultStr)
}

// AddInteraction adds a user message and assistant response to the session context
func (m *InMemoryManager) AddInteraction(sessionID, userMessage, assistantResponse string) error {
	if _, exists := m.contexts[sessionID]; !exists {
		m.contexts[sessionID] = make([]string, 0)
	}

	m.contexts[sessionID] = append(m.contexts[sessionID], userMessage, assistantResponse)
	return nil
}

// GetContext retrieves the conversation context for a session, including project context at the top
func (m *InMemoryManager) GetContext(ctx context.Context, sessionID string) ([]string, error) {
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

// GetConversationContext builds a formatted conversation context string for AI prompts
func (m *InMemoryManager) GetConversationContext(ctx context.Context, sessionID string, maxPairs int) (string, error) {
	contextData, err := m.GetContext(ctx, sessionID)
	if err != nil {
		return "", err
	}
	
	// Context comes as alternating user/assistant messages
	// Only include complete pairs
	totalMessages := len(contextData)
	if totalMessages%2 != 0 {
		totalMessages-- // Remove incomplete pair
	}
	
	if totalMessages == 0 {
		return "", nil
	}
	
	pairsToInclude := maxPairs
	if totalMessages/2 < maxPairs {
		pairsToInclude = totalMessages / 2
	}
	
	startIdx := totalMessages - (pairsToInclude * 2)
	
	var contextBuilder strings.Builder
	for i := startIdx; i < totalMessages; i += 2 {
		contextBuilder.WriteString(fmt.Sprintf("User: %s\n", contextData[i]))
		contextBuilder.WriteString(fmt.Sprintf("Assistant: %s\n", contextData[i+1]))
	}
	
	return strings.TrimSpace(contextBuilder.String()), nil
}

// GetLLMContext returns formatted context for LLM with internal default window size
func (m *InMemoryManager) GetLLMContext(ctx context.Context, sessionID string) (string, error) {
	// Use internal default of 5 pairs - this can become configurable later
	const defaultMaxPairs = 5
	return m.GetConversationContext(ctx, sessionID, defaultMaxPairs)
}

// ClearContext removes all context for a session
func (m *InMemoryManager) ClearContext(sessionID string) error {
	delete(m.contexts, sessionID)
	return nil
}
