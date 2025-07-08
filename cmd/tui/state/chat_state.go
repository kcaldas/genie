package state

import (
	"sync"
	"time"

	"github.com/kcaldas/genie/cmd/tui/types"
)

type ChatState struct {
	mu                  sync.RWMutex
	messages            []types.Message
	loading             bool
	waitingConfirmation bool
	loadingStartTime    time.Time
	maxMessages         int
}

func NewChatState(maxMessages int) *ChatState {
	if maxMessages <= 0 {
		maxMessages = 500 // Default fallback
	}
	return &ChatState{
		messages:            []types.Message{},
		loading:             false,
		waitingConfirmation: false,
		maxMessages:         maxMessages,
	}
}

func (s *ChatState) Lock() {
	s.mu.Lock()
}

func (s *ChatState) Unlock() {
	s.mu.Unlock()
}

func (s *ChatState) RLock() {
	s.mu.RLock()
}

func (s *ChatState) RUnlock() {
	s.mu.RUnlock()
}

func (s *ChatState) GetMessages() []types.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messagesCopy := make([]types.Message, len(s.messages))
	copy(messagesCopy, s.messages)
	return messagesCopy
}

func (s *ChatState) AddMessage(msg types.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, msg)

	if len(s.messages) > s.maxMessages {
		s.messages = s.messages[len(s.messages)-s.maxMessages:]
	}
}

func (s *ChatState) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = []types.Message{}
}

func (s *ChatState) IsLoading() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loading
}

func (s *ChatState) SetLoading(loading bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loading = loading
	if loading {
		s.loadingStartTime = time.Now()
	}
}

func (s *ChatState) GetMessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}

func (s *ChatState) GetLastMessage() *types.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.messages) == 0 {
		return nil
	}

	lastMsg := s.messages[len(s.messages)-1]
	return &lastMsg
}

func (s *ChatState) IsWaitingConfirmation() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.waitingConfirmation
}

func (s *ChatState) SetWaitingConfirmation(waiting bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waitingConfirmation = waiting
}

func (s *ChatState) GetLoadingDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.loading {
		return 0
	}
	return time.Since(s.loadingStartTime)
}
