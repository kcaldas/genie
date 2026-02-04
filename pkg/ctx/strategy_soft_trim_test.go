package ctx

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSoftTrimStrategy_Name(t *testing.T) {
	s := NewSoftTrimStrategy(100, 100)
	assert.Equal(t, "soft_trim", s.Name())
}

func TestSoftTrimStrategy_ContentWithinBudget(t *testing.T) {
	s := NewSoftTrimStrategy(100, 100)
	content := "Short content that fits easily."

	trimmed, tokens := s.Apply(content, 10000)

	assert.Equal(t, content, trimmed)
	assert.Equal(t, EstimateTokens(content), tokens)
}

func TestSoftTrimStrategy_TrimsMiddle(t *testing.T) {
	s := NewSoftTrimStrategy(20, 20)
	// Create content: 20 chars head + lots of middle + 20 chars tail
	head := strings.Repeat("H", 20)
	middle := strings.Repeat("M", 500)
	tail := strings.Repeat("T", 20)
	content := head + middle + tail

	// Budget that's smaller than the full content but larger than head+tail
	trimmed, tokens := s.Apply(content, 50)

	// Should contain head and tail
	assert.True(t, strings.HasPrefix(trimmed, head))
	assert.True(t, strings.HasSuffix(trimmed, tail))

	// Should contain the omission marker
	assert.Contains(t, trimmed, "omitted")
	assert.Contains(t, trimmed, "500 characters")

	// Should fit in budget
	assert.LessOrEqual(t, tokens, 50)
}

func TestSoftTrimStrategy_EmptyContent(t *testing.T) {
	s := NewSoftTrimStrategy(100, 100)

	trimmed, tokens := s.Apply("", 100)

	assert.Equal(t, "", trimmed)
	assert.Equal(t, 0, tokens)
}

func TestSoftTrimStrategy_ZeroBudget(t *testing.T) {
	s := NewSoftTrimStrategy(100, 100)

	trimmed, tokens := s.Apply("some content", 0)

	assert.Equal(t, "", trimmed)
	assert.Equal(t, 0, tokens)
}

func TestSoftTrimStrategy_ContentShorterThanHeadPlusTail(t *testing.T) {
	s := NewSoftTrimStrategy(100, 100)
	content := "Very short" // 10 chars, way less than 200

	trimmed, tokens := s.Apply(content, 10000)

	// Should return unchanged
	assert.Equal(t, content, trimmed)
	assert.Equal(t, EstimateTokens(content), tokens)
}

func TestSoftTrimStrategy_LargeContent(t *testing.T) {
	s := NewSoftTrimStrategy(1500, 1500)
	// Simulate a large file: 10000 chars
	content := strings.Repeat("package main\nimport \"fmt\"\n", 50) +
		strings.Repeat("func doStuff() { /* logic */ }\n", 200) +
		strings.Repeat("func main() { doStuff() }\n", 50)

	// Budget for about 1000 tokens
	trimmed, tokens := s.Apply(content, 1000)

	assert.LessOrEqual(t, tokens, 1000)
	assert.Greater(t, len(trimmed), 0)
	assert.Contains(t, trimmed, "omitted")
}

func TestSoftTrimStrategy_PreservesHeadAndTail(t *testing.T) {
	s := NewSoftTrimStrategy(10, 10)
	content := "ABCDEFGHIJ" + strings.Repeat("x", 100) + "0123456789"

	trimmed, _ := s.Apply(content, 20)

	assert.True(t, strings.HasPrefix(trimmed, "ABCDEFGHIJ"))
	assert.True(t, strings.HasSuffix(trimmed, "0123456789"))
}
