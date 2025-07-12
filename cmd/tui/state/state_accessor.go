package state

import (
	"time"

	"github.com/kcaldas/genie/cmd/tui/types"
)

type StateAccessor struct {
	chatState *ChatState
	uiState   *UIState
}

func NewStateAccessor(chatState *ChatState, uiState *UIState) *StateAccessor {
	return &StateAccessor{
		chatState: chatState,
		uiState:   uiState,
	}
}

func (s *StateAccessor) GetMessages() []types.Message {
	return s.chatState.GetMessages()
}

func (s *StateAccessor) AddMessage(msg types.Message) {
	s.chatState.AddMessage(msg)
}

func (s *StateAccessor) ClearMessages() {
	s.chatState.ClearMessages()
}

func (s *StateAccessor) IsLoading() bool {
	return s.chatState.IsLoading()
}

func (s *StateAccessor) SetLoading(loading bool) {
	s.chatState.SetLoading(loading)
}

// Debug methods removed - use types.Logger interface instead

func (s *StateAccessor) IsWaitingConfirmation() bool {
	return s.chatState.IsWaitingConfirmation()
}

func (s *StateAccessor) SetWaitingConfirmation(waiting bool) {
	s.chatState.SetWaitingConfirmation(waiting)
}

func (s *StateAccessor) GetLoadingDuration() time.Duration {
	return s.chatState.GetLoadingDuration()
}

func (s *StateAccessor) SetFocusedPanel(panelName string) {
	s.uiState.SetFocusedPanel(panelName)
}

// UI state management methods
func (s *StateAccessor) GetActiveConfirmationType() string {
	return s.uiState.GetActiveConfirmationType()
}

func (s *StateAccessor) SetActiveConfirmationType(confirmationType string) {
	s.uiState.SetActiveConfirmationType(confirmationType)
}

func (s *StateAccessor) GetCurrentDialog() types.Component {
	return s.uiState.GetCurrentDialog()
}

func (s *StateAccessor) SetCurrentDialog(dialog types.Component) {
	s.uiState.SetCurrentDialog(dialog)
}

func (s *StateAccessor) IsContextViewerActive() bool {
	return s.uiState.IsContextViewerActive()
}

func (s *StateAccessor) SetContextViewerActive(active bool) {
	s.uiState.SetContextViewerActive(active)
}
