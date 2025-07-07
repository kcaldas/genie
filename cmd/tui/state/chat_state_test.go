package state

import (
	"fmt"
	"sync"
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatState_NewChatState(t *testing.T) {
	state := NewChatState()

	assert.NotNil(t, state)
	assert.Empty(t, state.GetMessages())
	assert.False(t, state.IsLoading())
	assert.Equal(t, 0, state.GetMessageCount())
}

func TestChatState_AddMessage(t *testing.T) {
	scenarios := []struct {
		name     string
		messages []types.Message
		expected int
	}{
		{
			name:     "single message",
			messages: []types.Message{{Role: "user", Content: "hello"}},
			expected: 1,
		},
		{
			name: "multiple messages",
			messages: []types.Message{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi there"},
				{Role: "user", Content: "how are you?"},
			},
			expected: 3,
		},
		{
			name:     "empty messages",
			messages: []types.Message{},
			expected: 0,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			state := NewChatState()

			for _, msg := range s.messages {
				state.AddMessage(msg)
			}

			assert.Equal(t, s.expected, state.GetMessageCount())

			retrievedMessages := state.GetMessages()
			assert.Len(t, retrievedMessages, s.expected)

			for i, msg := range s.messages {
				assert.Equal(t, msg.Role, retrievedMessages[i].Role)
				assert.Equal(t, msg.Content, retrievedMessages[i].Content)
			}
		})
	}
}

func TestChatState_GetMessages_ReturnsCopy(t *testing.T) {
	state := NewChatState()
	originalMsg := types.Message{Role: "user", Content: "original"}
	state.AddMessage(originalMsg)

	// Get messages and modify the returned slice
	messages := state.GetMessages()
	require.Len(t, messages, 1)

	// Modify the returned slice
	messages[0].Content = "modified"
	messages = append(messages, types.Message{Role: "hacker", Content: "injected"})

	// Verify original state is unchanged
	originalMessages := state.GetMessages()
	require.Len(t, originalMessages, 1)
	assert.Equal(t, "original", originalMessages[0].Content)
	assert.Equal(t, "user", originalMessages[0].Role)
}

func TestChatState_ClearMessages(t *testing.T) {
	state := NewChatState()

	// Add some messages
	state.AddMessage(types.Message{Role: "user", Content: "test1"})
	state.AddMessage(types.Message{Role: "assistant", Content: "test2"})

	assert.Equal(t, 2, state.GetMessageCount())

	// Clear messages
	state.ClearMessages()

	assert.Equal(t, 0, state.GetMessageCount())
	assert.Empty(t, state.GetMessages())
}

func TestChatState_LoadingState(t *testing.T) {
	state := NewChatState()

	// Initially not loading
	assert.False(t, state.IsLoading())

	// Set loading
	state.SetLoading(true)
	assert.True(t, state.IsLoading())

	// Unset loading
	state.SetLoading(false)
	assert.False(t, state.IsLoading())
}

func TestChatState_GetLastMessage(t *testing.T) {
	scenarios := []struct {
		name     string
		messages []types.Message
		expected *types.Message
	}{
		{
			name:     "no messages",
			messages: []types.Message{},
			expected: nil,
		},
		{
			name:     "single message",
			messages: []types.Message{{Role: "user", Content: "hello"}},
			expected: &types.Message{Role: "user", Content: "hello"},
		},
		{
			name: "multiple messages",
			messages: []types.Message{
				{Role: "user", Content: "first"},
				{Role: "assistant", Content: "second"},
				{Role: "user", Content: "last"},
			},
			expected: &types.Message{Role: "user", Content: "last"},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			state := NewChatState()

			for _, msg := range s.messages {
				state.AddMessage(msg)
			}

			lastMsg := state.GetLastMessage()
			if s.expected == nil {
				assert.Nil(t, lastMsg)
			} else {
				require.NotNil(t, lastMsg)
				assert.Equal(t, s.expected.Role, lastMsg.Role)
				assert.Equal(t, s.expected.Content, lastMsg.Content)
			}
		})
	}
}

func TestChatState_ConcurrentAccess(t *testing.T) {
	state := NewChatState()

	// Test concurrent reads and writes
	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 100

	// Start writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := types.Message{
					Role:    "user",
					Content: fmt.Sprintf("writer-%d-msg-%d", writerID, j),
				}
				state.AddMessage(msg)
			}
		}(i)
	}

	// Start readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				_ = state.GetMessages()
				_ = state.GetMessageCount()
				_ = state.IsLoading()
				state.SetLoading(j%2 == 0)
			}
		}()
	}

	wg.Wait()

	// Verify final state
	finalCount := state.GetMessageCount()
	assert.Equal(t, maxMessages, finalCount)

	// Verify all messages are intact
	messages := state.GetMessages()
	assert.Len(t, messages, maxMessages)
}
