package ai

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// SharedMockGen provides a unified mock implementation for Gen interface
// This replaces the duplicate mock implementations that were in chain_test.go and prompt_exec_test.go
type SharedMockGen struct {
	mock.Mock
	// Optional: additional fields for tracking when using response queue mode
	UsedPrompts   []Prompt
	LastAttrs     []Attr
	CallCounts    map[string]int
	ResponseQueue []string
}

func NewSharedMockGen() *SharedMockGen {
	return &SharedMockGen{
		CallCounts: make(map[string]int),
	}
}

func (m *SharedMockGen) GenerateContent(prompt Prompt, debug bool, args ...string) (string, error) {
	if m.CallCounts != nil {
		m.CallCounts["GenerateContent"]++
	}

	// If using response queue mode
	if len(m.ResponseQueue) > 0 {
		resp := m.ResponseQueue[0]
		m.ResponseQueue = m.ResponseQueue[1:]
		if resp == "ERROR" {
			return "", errors.New("mock error")
		}
		return resp, nil
	}

	// Otherwise use testify/mock
	mockArgs := m.Called(prompt, debug, args)
	return mockArgs.String(0), mockArgs.Error(1)
}

func (m *SharedMockGen) GenerateContentAttr(prompt Prompt, debug bool, attrs []Attr) (string, error) {
	if m.CallCounts != nil {
		m.CallCounts["GenerateContentAttr"]++
	}

	// Track prompts and attributes when in tracking mode
	if m.UsedPrompts != nil {
		m.UsedPrompts = append(m.UsedPrompts, prompt)
	}
	if len(attrs) > 0 {
		m.LastAttrs = attrs
	}

	// If using response queue mode
	if len(m.ResponseQueue) > 0 {
		resp := m.ResponseQueue[0]
		m.ResponseQueue = m.ResponseQueue[1:]
		if resp == "ERROR" {
			return "", errors.New("mock error")
		}
		return resp, nil
	}

	// Otherwise use testify/mock
	mockArgs := m.Called(prompt, debug, attrs)
	return mockArgs.String(0), mockArgs.Error(1)
}

func TestSharedMockGen_GenerateContent(t *testing.T) {
	mockGen := new(SharedMockGen)
	testPrompt := Prompt{Text: "test"}

	mockGen.On("GenerateContent", testPrompt, true, []string{"arg1"}).Return("mocked response", nil)

	result, err := mockGen.GenerateContent(testPrompt, true, "arg1")

	assert.NoError(t, err)
	assert.Equal(t, "mocked response", result)
	mockGen.AssertExpectations(t)
}

func TestSharedMockGen_GenerateContentAttr(t *testing.T) {
	mockGen := new(SharedMockGen)
	testPrompt := Prompt{Text: "test"}
	testAttrs := []Attr{{Key: "key1", Value: "value1"}}

	mockGen.On("GenerateContentAttr", testPrompt, false, testAttrs).Return("attr response", nil)

	result, err := mockGen.GenerateContentAttr(testPrompt, false, testAttrs)

	assert.NoError(t, err)
	assert.Equal(t, "attr response", result)
	mockGen.AssertExpectations(t)
}

func TestSharedMockGen_ResponseQueue(t *testing.T) {
	mockGen := NewSharedMockGen()
	mockGen.ResponseQueue = []string{"response1", "response2", "ERROR"}
	mockGen.UsedPrompts = []Prompt{}

	testPrompt := Prompt{Text: "test"}
	testAttrs := []Attr{{Key: "key1", Value: "value1"}}

	// First call
	result1, err1 := mockGen.GenerateContentAttr(testPrompt, false, testAttrs)
	assert.NoError(t, err1)
	assert.Equal(t, "response1", result1)
	assert.Equal(t, 1, mockGen.CallCounts["GenerateContentAttr"])
	assert.Equal(t, 1, len(mockGen.UsedPrompts))

	// Second call
	result2, err2 := mockGen.GenerateContentAttr(testPrompt, false, testAttrs)
	assert.NoError(t, err2)
	assert.Equal(t, "response2", result2)
	assert.Equal(t, 2, mockGen.CallCounts["GenerateContentAttr"])

	// Third call should error
	result3, err3 := mockGen.GenerateContentAttr(testPrompt, false, testAttrs)
	assert.Error(t, err3)
	assert.Equal(t, "", result3)
	assert.Equal(t, 3, mockGen.CallCounts["GenerateContentAttr"])
}
