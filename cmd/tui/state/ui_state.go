package state

import (
	"sync"
)

type UIState struct {
	mu           sync.RWMutex
	focusedPanel string // Changed to panel name (e.g., "input", "messages", "debug")
	debugVisible bool
}

func NewUIState() *UIState {
	return &UIState{
		focusedPanel: "input", // Default to input panel
		debugVisible: false,
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

func (s *UIState) GetFocusedPanel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.focusedPanel
}

func (s *UIState) SetFocusedPanel(panelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.focusedPanel = panelName
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