package history

import "fmt"

// HistoryManager manages complete conversation history for sessions
type HistoryManager interface {
	AddInteraction(sessionID, userMessage, assistantResponse string) error
	GetHistory(sessionID string) ([]string, error)
	ClearHistory(sessionID string) error
}

// InMemoryManager implements HistoryManager with in-memory storage
type InMemoryManager struct {
	histories map[string][]string
}

// NewHistoryManager creates a new in-memory history manager
func NewHistoryManager() HistoryManager {
	return &InMemoryManager{
		histories: make(map[string][]string),
	}
}

// AddInteraction adds a user message and assistant response to the session history
func (m *InMemoryManager) AddInteraction(sessionID, userMessage, assistantResponse string) error {
	if _, exists := m.histories[sessionID]; !exists {
		m.histories[sessionID] = make([]string, 0)
	}
	
	m.histories[sessionID] = append(m.histories[sessionID], userMessage, assistantResponse)
	return nil
}

// GetHistory retrieves the complete conversation history for a session
func (m *InMemoryManager) GetHistory(sessionID string) ([]string, error) {
	history, exists := m.histories[sessionID]
	if !exists {
		return nil, fmt.Errorf("history for session %s not found", sessionID)
	}
	
	// Return a copy to prevent external modification
	result := make([]string, len(history))
	copy(result, history)
	return result, nil
}

// ClearHistory removes all history for a session
func (m *InMemoryManager) ClearHistory(sessionID string) error {
	delete(m.histories, sessionID)
	return nil
}