package state

import (
	"sync"
	
	"github.com/kcaldas/genie/cmd/tui/types"
)

type UIState struct {
	mu           sync.RWMutex
	focusedPanel string // Changed to panel name (e.g., "input", "messages", "debug")
	debugVisible bool
	
	// Confirmation state
	activeConfirmationType string // "tool" or "user" or ""
	
	// Dialog management
	currentDialog types.Component
	
	// Context viewer state
	contextViewerActive bool
}

func NewUIState() *UIState {
	return &UIState{
		focusedPanel:           "input", // Default to input panel
		debugVisible:           false,
		activeConfirmationType: "",      // No active confirmation initially
		currentDialog:          nil,     // No dialog initially
		contextViewerActive:    false,   // Context viewer not active initially
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

// Confirmation type management
func (s *UIState) GetActiveConfirmationType() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeConfirmationType
}

func (s *UIState) SetActiveConfirmationType(confirmationType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeConfirmationType = confirmationType
}

// Dialog management
func (s *UIState) GetCurrentDialog() types.Component {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentDialog
}

func (s *UIState) SetCurrentDialog(dialog types.Component) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentDialog = dialog
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