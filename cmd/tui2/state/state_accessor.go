package state

import (
	"time"

	"github.com/kcaldas/genie/cmd/tui2/types"
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

func (s *StateAccessor) GetDebugMessages() []string {
	return s.uiState.GetDebugMessages()
}

func (s *StateAccessor) AddDebugMessage(msg string) {
	s.uiState.AddDebugMessage(msg)
}

func (s *StateAccessor) IsWaitingConfirmation() bool {
	return s.chatState.IsWaitingConfirmation()
}

func (s *StateAccessor) SetWaitingConfirmation(waiting bool) {
	s.chatState.SetWaitingConfirmation(waiting)
}

func (s *StateAccessor) GetLoadingDuration() time.Duration {
	return s.chatState.GetLoadingDuration()
}