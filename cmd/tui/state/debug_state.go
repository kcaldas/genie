package state

import (
	"sync"
)

// DebugState manages debug display buffer for F12 panel
// Note: This is only for displaying debug content in the TUI panel.
// Actual debug logging is handled by the centralized logging system.
type DebugState struct {
	mu            sync.RWMutex
	messages      []string
	maxMessages   int
}

// NewDebugState creates a new debug state
func NewDebugState() *DebugState {
	return &DebugState{
		messages:    []string{},
		maxMessages: 1000,
	}
}

// GetDebugMessages returns a copy of all debug messages
func (s *DebugState) GetDebugMessages() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	messagesCopy := make([]string, len(s.messages))
	copy(messagesCopy, s.messages)
	return messagesCopy
}

// AddDebugMessage adds a message to the display buffer (used for F12 panel display)
func (s *DebugState) AddDebugMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.messages = append(s.messages, msg)
	
	// Trim old messages if we exceed the limit
	if len(s.messages) > s.maxMessages {
		// Keep the last 90% of messages
		keepFrom := s.maxMessages / 10
		s.messages = s.messages[keepFrom:]
	}
}

// ClearDebugMessages clears all debug messages
func (s *DebugState) ClearDebugMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = []string{}
}


// SetMaxMessages sets the maximum number of debug messages to keep
func (s *DebugState) SetMaxMessages(max int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if max > 0 {
		s.maxMessages = max
	}
}