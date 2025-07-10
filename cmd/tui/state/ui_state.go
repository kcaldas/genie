package state

import (
	"sync"

	"github.com/kcaldas/genie/cmd/tui/types"
)

type UIState struct {
	mu           sync.RWMutex
	focusedPanel types.FocusablePanel
	debugVisible bool
	config       *types.Config
}

func NewUIState(config *types.Config) *UIState {
	return &UIState{
		focusedPanel: types.PanelInput,
		debugVisible: false,
		config:       config,
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

func (s *UIState) GetFocusedPanel() types.FocusablePanel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.focusedPanel
}

func (s *UIState) SetFocusedPanel(panel types.FocusablePanel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.focusedPanel = panel
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

func (s *UIState) GetConfig() *types.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *UIState) UpdateConfig(fn func(*types.Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.config)
}