package state

import (
	"sync"
)

type UIState struct {
	mu           sync.RWMutex
	debugVisible bool

	// Context viewer state
	contextViewerActive bool
}

func NewUIState() *UIState {
	return &UIState{
		debugVisible:        false,
		contextViewerActive: false, // Context viewer not active initially
	}
}

func (s *UIState) Lock() {
	s.mu.Lock()
}

func (s *UIState) Unlock() {
	s.mu.Unlock()
}

func (s *UIState) RLock() {
	s.mu.RLock()
}

func (s *UIState) RUnlock() {
	s.mu.RUnlock()
}

func (s *UIState) IsDebugVisible() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.debugVisible
}

func (s *UIState) SetDebugVisible(visible bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.debugVisible = visible
}

func (s *UIState) ToggleDebugVisible() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.debugVisible = !s.debugVisible
}

// Context viewer state management
func (s *UIState) IsContextViewerActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.contextViewerActive
}

func (s *UIState) SetContextViewerActive(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextViewerActive = active
}
