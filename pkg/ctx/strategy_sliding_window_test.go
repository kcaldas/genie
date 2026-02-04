package ctx

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func formatMessage(m Message) string {
	var parts []string
	if m.User != "" {
		parts = append(parts, "User: "+m.User)
	}
	if m.Assistant != "" {
		parts = append(parts, "Assistant: "+m.Assistant)
	}
	return strings.Join(parts, "\n")
}

func TestSlidingWindowStrategy_Name(t *testing.T) {
	s := NewSlidingWindowStrategy()
	assert.Equal(t, "sliding_window", s.Name())
}

func TestSlidingWindowStrategy_EmptyMessages(t *testing.T) {
	s := NewSlidingWindowStrategy()

	kept, tokens := s.ApplyToCollection(nil, 1000, formatMessage)

	assert.Nil(t, kept)
	assert.Equal(t, 0, tokens)
}

func TestSlidingWindowStrategy_ZeroBudget(t *testing.T) {
	s := NewSlidingWindowStrategy()
	msgs := []Message{{User: "hi", Assistant: "hello"}}

	kept, tokens := s.ApplyToCollection(msgs, 0, formatMessage)

	assert.Nil(t, kept)
	assert.Equal(t, 0, tokens)
}

func TestSlidingWindowStrategy_AllFit(t *testing.T) {
	s := NewSlidingWindowStrategy()
	msgs := []Message{
		{User: "Q1", Assistant: "A1"},
		{User: "Q2", Assistant: "A2"},
	}

	kept, tokens := s.ApplyToCollection(msgs, 10000, formatMessage)

	assert.Equal(t, 2, len(kept))
	assert.Equal(t, "Q1", kept[0].User)
	assert.Equal(t, "Q2", kept[1].User)
	assert.Greater(t, tokens, 0)
}

func TestSlidingWindowStrategy_KeepsMostRecent(t *testing.T) {
	s := NewSlidingWindowStrategy()

	// Create messages where each is roughly 10 tokens (40 chars)
	msgs := make([]Message, 10)
	for i := range msgs {
		msgs[i] = Message{
			User:      fmt.Sprintf("Question %d padding", i),
			Assistant: fmt.Sprintf("Answer %d with padding", i),
		}
	}

	// Budget for roughly 3 messages
	// Each message formatted: "User: Question X padding\nAssistant: Answer X with padding"
	// ~55 chars = ~14 tokens per message. Budget for 3 = ~42 tokens
	singleFormatted := formatMessage(msgs[0])
	singleTokens := EstimateTokens(singleFormatted)
	budget := singleTokens * 3

	kept, tokens := s.ApplyToCollection(msgs, budget, formatMessage)

	// Should keep the last 3 messages
	assert.Equal(t, 3, len(kept))
	assert.Equal(t, msgs[7].User, kept[0].User)
	assert.Equal(t, msgs[8].User, kept[1].User)
	assert.Equal(t, msgs[9].User, kept[2].User)
	assert.LessOrEqual(t, tokens, budget)
}

func TestSlidingWindowStrategy_PreservesOrder(t *testing.T) {
	s := NewSlidingWindowStrategy()
	msgs := []Message{
		{User: "oldest", Assistant: "a1"},
		{User: "middle", Assistant: "a2"},
		{User: "newest", Assistant: "a3"},
	}

	// Budget for 2 messages
	singleTokens := EstimateTokens(formatMessage(msgs[0]))
	budget := singleTokens * 2

	kept, _ := s.ApplyToCollection(msgs, budget, formatMessage)

	assert.Equal(t, 2, len(kept))
	// Should be in chronological order: middle, newest
	assert.Equal(t, "middle", kept[0].User)
	assert.Equal(t, "newest", kept[1].User)
}

func TestSlidingWindowStrategy_SingleMessageTooLarge(t *testing.T) {
	s := NewSlidingWindowStrategy()
	msgs := []Message{
		{User: strings.Repeat("x", 1000), Assistant: strings.Repeat("y", 1000)},
	}

	kept, tokens := s.ApplyToCollection(msgs, 1, formatMessage)

	// Single message doesn't fit in budget of 1 token
	assert.Nil(t, kept)
	assert.Equal(t, 0, tokens)
}

func TestSlidingWindowStrategy_SingleMessageFits(t *testing.T) {
	s := NewSlidingWindowStrategy()
	msgs := []Message{
		{User: "hi", Assistant: "hello"},
	}

	kept, tokens := s.ApplyToCollection(msgs, 1000, formatMessage)

	assert.Equal(t, 1, len(kept))
	assert.Equal(t, "hi", kept[0].User)
	assert.Greater(t, tokens, 0)
}

func TestSlidingWindowStrategy_DoesNotMutateInput(t *testing.T) {
	s := NewSlidingWindowStrategy()
	msgs := []Message{
		{User: "Q1", Assistant: "A1"},
		{User: "Q2", Assistant: "A2"},
		{User: "Q3", Assistant: "A3"},
	}

	original := make([]Message, len(msgs))
	copy(original, msgs)

	singleTokens := EstimateTokens(formatMessage(msgs[0]))
	s.ApplyToCollection(msgs, singleTokens, formatMessage)

	// Original slice should not be modified
	assert.Equal(t, original, msgs)
}
