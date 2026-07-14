package state

import (
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Messages need stable identifiers: index-based updates silently target
// the wrong message once the sliding window evicts older entries.
func TestChatState_AddMessageAssignsStableIDs(t *testing.T) {
	s := NewChatState(10)

	id1 := s.AddMessage(types.Message{Role: "user", Content: "one"})
	id2 := s.AddMessage(types.Message{Role: "assistant", Content: "two"})

	assert.NotEqual(t, id1, id2, "each message must get a unique ID")

	msgs := s.GetMessages()
	require.Len(t, msgs, 2)
	assert.Equal(t, id1, msgs[0].ID)
	assert.Equal(t, id2, msgs[1].ID)
}

func TestChatState_UpdateMessageByID(t *testing.T) {
	s := NewChatState(10)

	s.AddMessage(types.Message{Role: "user", Content: "one"})
	id := s.AddMessage(types.Message{Role: "assistant", Content: "streaming..."})

	ok := s.UpdateMessageByID(id, func(m *types.Message) {
		m.Content = "final"
	})
	require.True(t, ok)

	msgs := s.GetMessages()
	assert.Equal(t, "final", msgs[1].Content)
}

func TestChatState_UpdateMessageByIDSurvivesWindowSlide(t *testing.T) {
	s := NewChatState(3)

	s.AddMessage(types.Message{Role: "user", Content: "old-1"})
	s.AddMessage(types.Message{Role: "user", Content: "old-2"})
	target := s.AddMessage(types.Message{Role: "assistant", Content: "streaming..."})

	// Force the window to slide: "old-1" is evicted and indices shift.
	s.AddMessage(types.Message{Role: "user", Content: "new-1"})

	ok := s.UpdateMessageByID(target, func(m *types.Message) {
		m.Content = "final"
	})
	require.True(t, ok, "ID-based update must survive eviction of older messages")

	msgs := s.GetMessages()
	require.Len(t, msgs, 3)
	assert.Equal(t, "final", msgs[1].Content, "the streamed message, not its old index, must be updated")
}

func TestChatState_UpdateMessageByIDReturnsFalseForEvicted(t *testing.T) {
	s := NewChatState(2)

	evicted := s.AddMessage(types.Message{Role: "user", Content: "old"})
	s.AddMessage(types.Message{Role: "user", Content: "a"})
	s.AddMessage(types.Message{Role: "user", Content: "b"}) // evicts "old"

	ok := s.UpdateMessageByID(evicted, func(m *types.Message) {
		m.Content = "should not happen"
	})
	assert.False(t, ok)
}

// Range reads must return copies: handing out sub-slices of the internal
// array lets a later append race with a reader holding the slice.
func TestChatState_RangeReadsReturnCopies(t *testing.T) {
	s := NewChatState(10)
	s.AddMessage(types.Message{Role: "user", Content: "one"})
	s.AddMessage(types.Message{Role: "user", Content: "two"})

	got := s.GetLastMessages(2)
	require.Len(t, got, 2)
	got[0].Content = "mutated"

	assert.Equal(t, "one", s.GetMessages()[0].Content, "GetLastMessages must not alias internal storage")

	ranged := s.GetMessageRange(0, 2)
	require.Len(t, ranged, 2)
	ranged[1].Content = "mutated"
	assert.Equal(t, "two", s.GetMessages()[1].Content, "GetMessageRange must not alias internal storage")
}
