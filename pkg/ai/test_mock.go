package ai

import (
	"context"
	"fmt"
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