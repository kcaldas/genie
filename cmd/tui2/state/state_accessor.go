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

func (s *StateAccessor) ClearDebugMessages() {
	s.uiState.ClearDebugMessages()
}

// Message range access methods for yank functionality
func (s *StateAccessor) GetMessageCount() int {
	messages := s.chatState.GetMessages()
	return len(messages)
}

func (s *StateAccessor) GetMessageRange(start, count int) []types.Message {
	messages := s.chatState.GetMessages()
	if start < 0 || start >= len(messages) {
		return []types.Message{}
	}
	
	end := start + count
	if end > len(messages) {
		end = len(messages)
	}
	
	return messages[start:end]
}

func (s *StateAccessor) GetLastMessages(count int) []types.Message {
	messages := s.chatState.GetMessages()
	if count <= 0 {
		return []types.Message{}
	}
	
	start := len(messages) - count
	if start < 0 {
		start = 0
	}
	
	return messages[start:]
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