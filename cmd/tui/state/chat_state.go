package state

import (
	"sync"

	"github.com/kcaldas/genie/cmd/tui/types"
)

type ChatState struct {
	mu                  sync.RWMutex
	messages            []types.Message
	waitingConfirmation bool
	maxMessages         int
	nextID              int64
}

func NewChatState(maxMessages int) *ChatState {
	if maxMessages <= 0 {
		maxMessages = 500 // Default fallback
	}
	return &ChatState{
		messages:            []types.Message{},
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

// AddMessage appends a message, assigns it a stable ID, and returns
// that ID. Older messages may be evicted to honor maxMessages; IDs of
// surviving messages are unaffected.
func (s *ChatState) AddMessage(msg types.Message) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	msg.ID = s.nextID
	s.messages = append(s.messages, msg)

	if len(s.messages) > s.maxMessages {
		s.messages = s.messages[len(s.messages)-s.maxMessages:]
	}
	return msg.ID
}

func (s *ChatState) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = []types.Message{}
}

func (s *ChatState) GetMessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}

// UpdateMessageByID mutates the message with the given ID in place.
// It returns false when the message has been evicted or never existed.
// The update callback must not change the message's ID.
func (s *ChatState) UpdateMessageByID(id int64, update func(*types.Message)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Recent messages are the common target; scan from the tail.
	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].ID == id {
			if update != nil {
				update(&s.messages[i])
				s.messages[i].ID = id // the ID is not the caller's to change
			}
			return true
		}
	}
	return false
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

func (s *ChatState) GetMessageRange(start, count int) []types.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if start < 0 || start >= len(s.messages) {
		return []types.Message{}
	}

	end := start + count
	end = min(end, len(s.messages))

	out := make([]types.Message, end-start)
	copy(out, s.messages[start:end])
	return out
}

func (s *ChatState) GetLastMessages(count int) []types.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if count <= 0 {
		return []types.Message{}
	}

	start := len(s.messages) - count
	start = max(start, 0)

	out := make([]types.Message, len(s.messages)-start)
	copy(out, s.messages[start:])
	return out
}
