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
	state := NewChatState(100)

	assert.NotNil(t, state)
	assert.Empty(t, state.GetMessages())
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
			state := NewChatState(100)

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
	state := NewChatState(100)
	originalMsg := types.Message{Role: "user", Content: "original"}
	state.AddMessage(originalMsg)

	// Get messages and modify the returned slice
	messages := state.GetMessages()
	require.Len(t, messages, 1)

	// Modify the returned slice
	messages[0].Content = "modified"
	_ = append(messages, types.Message{Role: "hacker", Content: "injected"})

	// Verify original state is unchanged
	originalMessages := state.GetMessages()
	require.Len(t, originalMessages, 1)
	assert.Equal(t, "original", originalMessages[0].Content)
	assert.Equal(t, "user", originalMessages[0].Role)
}

func TestChatState_ClearMessages(t *testing.T) {
	state := NewChatState(100)

	// Add some messages
	state.AddMessage(types.Message{Role: "user", Content: "test1"})
	state.AddMessage(types.Message{Role: "assistant", Content: "test2"})

	assert.Equal(t, 2, state.GetMessageCount())

	// Clear messages
	state.ClearMessages()

	assert.Equal(t, 0, state.GetMessageCount())
	assert.Empty(t, state.GetMessages())
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
			state := NewChatState(100)

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
	state := NewChatState(100)

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
			}
		}()
	}

	wg.Wait()

	// Verify final state
	finalCount := state.GetMessageCount()
	assert.Equal(t, 100, finalCount)

	// Verify all messages are intact
	messages := state.GetMessages()
	assert.Len(t, messages, 100)
}

func TestChatState_ConfigurableMaxMessages(t *testing.T) {
	testCases := []struct {
		name        string
		maxMessages int
		addMessages int
		expected    int
	}{
		{
			name:        "default 100 messages",
			maxMessages: 100,
			addMessages: 150,
			expected:    100,
		},
		{
			name:        "small limit of 5 messages",
			maxMessages: 5,
			addMessages: 10,
			expected:    5,
		},
		{
			name:        "zero max messages defaults to 500",
			maxMessages: 0,
			addMessages: 600,
			expected:    500,
		},
		{
			name:        "negative max messages defaults to 500",
			maxMessages: -10,
			addMessages: 600,
			expected:    500,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewChatState(tc.maxMessages)

			// Add more messages than the limit
			for i := 0; i < tc.addMessages; i++ {
				state.AddMessage(types.Message{
					Role:    "user",
					Content: fmt.Sprintf("message %d", i),
				})
			}

			// Should only keep the configured maximum
			assert.Equal(t, tc.expected, state.GetMessageCount())

			// Verify we have the most recent messages
			messages := state.GetMessages()
			if len(messages) > 0 {
				lastMessage := messages[len(messages)-1]
				expectedLastContent := fmt.Sprintf("message %d", tc.addMessages-1)
				assert.Equal(t, expectedLastContent, lastMessage.Content)
			}
		})
	}
}
