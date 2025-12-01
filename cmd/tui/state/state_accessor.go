package state

import (
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

func (s *StateAccessor) GetMessageCount() int {
	return s.chatState.GetMessageCount()
}

func (s *StateAccessor) UpdateMessage(index int, update func(*types.Message)) bool {
	return s.chatState.UpdateMessage(index, update)
}

func (s *StateAccessor) IsWaitingConfirmation() bool {
	return s.chatState.IsWaitingConfirmation()
}

func (s *StateAccessor) SetWaitingConfirmation(waiting bool) {
	s.chatState.SetWaitingConfirmation(waiting)
}

func (s *StateAccessor) IsContextViewerActive() bool {
	return s.uiState.IsContextViewerActive()
}

func (s *StateAccessor) SetContextViewerActive(active bool) {
	s.uiState.SetContextViewerActive(active)
}
