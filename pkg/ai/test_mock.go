package ai

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// MockGen implements the Gen interface for testing
type MockGen struct {
	ResponseQueue []string
	CallCounts    map[string]int
	UsedPrompts   []Prompt
	LastAttrs     []Attr
	currentIndex  int
}

// NewSharedMockGen creates a new mock generator for testing
func NewSharedMockGen() *MockGen {
	return &MockGen{
		ResponseQueue: make([]string, 0),
		CallCounts:    make(map[string]int),
		UsedPrompts:   make([]Prompt, 0),
		LastAttrs:     make([]Attr, 0),
		currentIndex:  0,
	}
}

// GenerateContent implements the Gen interface
func (m *MockGen) GenerateContent(ctx context.Context, prompt Prompt, debug bool, args ...string) (string, error) {
	m.CallCounts["GenerateContent"]++
	m.UsedPrompts = append(m.UsedPrompts, prompt)

	if m.currentIndex < len(m.ResponseQueue) {
		response := m.ResponseQueue[m.currentIndex]
		m.currentIndex++

		// Check if it's an error response
		if strings.HasPrefix(response, "ERROR") {
			return "", fmt.Errorf("mock error")
		}

		return response, nil
	}

	return "mock response", nil
}

// GenerateContentAttr implements the Gen interface
func (m *MockGen) GenerateContentAttr(ctx context.Context, prompt Prompt, debug bool, attrs []Attr) (string, error) {
	m.CallCounts["GenerateContentAttr"]++
	m.UsedPrompts = append(m.UsedPrompts, prompt)
	m.LastAttrs = attrs

	if m.currentIndex < len(m.ResponseQueue) {
		response := m.ResponseQueue[m.currentIndex]
		m.currentIndex++

		// Check if it's an error response
		if strings.HasPrefix(response, "ERROR") {
			return "", fmt.Errorf("mock error")
		}

		return response, nil
	}

	return "mock response", nil
}

// GenerateContentStream implements the streaming portion of the Gen interface.
// It reuses GenerateContent to keep behavior consistent.
func (m *MockGen) GenerateContentStream(ctx context.Context, prompt Prompt, debug bool, args ...string) (Stream, error) {
	response, err := m.GenerateContent(ctx, prompt, debug, args...)
	if err != nil {
		return nil, err
	}
	return newSliceStream(&StreamChunk{Text: response}), nil
}

// GenerateContentAttrStream implements the streaming interface with attrs.
func (m *MockGen) GenerateContentAttrStream(ctx context.Context, prompt Prompt, debug bool, attrs []Attr) (Stream, error) {
	response, err := m.GenerateContentAttr(ctx, prompt, debug, attrs)
	if err != nil {
		return nil, err
	}
	return newSliceStream(&StreamChunk{Text: response}), nil
}

func (m *MockGen) CountTokens(ctx context.Context, p Prompt, debug bool, args ...string) (*TokenCount, error) {
	// Mock implementation - simple token estimation
	textLength := len(p.Text) + len(p.Instruction)
	estimatedTokens := int32(textLength / 4)
	if estimatedTokens < 1 {
		estimatedTokens = 1
	}

	return &TokenCount{
		TotalTokens:  estimatedTokens,
		InputTokens:  estimatedTokens,
		OutputTokens: 0,
	}, nil
}

func (m *MockGen) GetStatus() *Status {
	return &Status{Connected: true, Backend: "mock-backend", Message: "Mock generator is connected"}
}

type sliceStream struct {
	chunks []*StreamChunk
	idx    int
	closed bool
}

func newSliceStream(chunks ...*StreamChunk) *sliceStream {
	return &sliceStream{
		chunks: chunks,
	}
}

func (s *sliceStream) Recv() (*StreamChunk, error) {
	if s.closed {
		return nil, io.EOF
	}
	if s.idx >= len(s.chunks) {
		s.closed = true
		return nil, io.EOF
	}
	chunk := s.chunks[s.idx]
	s.idx++
	if s.idx >= len(s.chunks) {
		s.closed = true
	}
	if chunk == nil {
		return &StreamChunk{}, nil
	}
	return chunk, nil
}

func (s *sliceStream) Close() error {
	s.closed = true
	return nil
}
