package state

import (
	"fmt"
	"sync"
	"time"
)

// DebugState manages debug-related state
type DebugState struct {
	mu            sync.RWMutex
	messages      []string
	debugMode     bool
	maxMessages   int
}

// NewDebugState creates a new debug state
func NewDebugState() *DebugState {
	return &DebugState{
		messages:    []string{},
		debugMode:   false,
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

// AddDebugMessage adds a new debug message with timestamp
func (s *DebugState) AddDebugMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Only add messages if debug mode is enabled
	if !s.debugMode {
		return
	}
	
	// Format message with timestamp
	timestampedMsg := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05.000"), msg)
	s.messages = append(s.messages, timestampedMsg)
	
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

// IsDebugMode returns whether debug mode is enabled
func (s *DebugState) IsDebugMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.debugMode
}

// SetDebugMode sets the debug mode
func (s *DebugState) SetDebugMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.debugMode = enabled
}

// SetMaxMessages sets the maximum number of debug messages to keep
func (s *DebugState) SetMaxMessages(max int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if max > 0 {
		s.maxMessages = max
	}
}