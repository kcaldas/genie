package state

import (
	"fmt"
	"sync"
	"time"

	"github.com/kcaldas/genie/cmd/tui2/types"
)

type UIState struct {
	mu            sync.RWMutex
	debugMessages []string
	focusedPanel  types.FocusablePanel
	debugVisible  bool
	config        *types.Config
}

func NewUIState(config *types.Config) *UIState {
	return &UIState{
		debugMessages: []string{},
		focusedPanel:  types.PanelInput,
		debugVisible:  false,
		config:        config,
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

func (s *UIState) GetDebugMessages() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	messagesCopy := make([]string, len(s.debugMessages))
	copy(messagesCopy, s.debugMessages)
	return messagesCopy
}

func (s *UIState) AddDebugMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Only add debug messages if debug is enabled
	if !s.config.DebugEnabled {
		return
	}
	
	// Format message with timestamp
	timestampedMsg := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05.000"), msg)
	s.debugMessages = append(s.debugMessages, timestampedMsg)
	
	if len(s.debugMessages) > 1000 {
		s.debugMessages = s.debugMessages[100:]
	}
}

func (s *UIState) ClearDebugMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.debugMessages = []string{}
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