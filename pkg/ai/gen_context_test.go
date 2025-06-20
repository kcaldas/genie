package ai_test

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGenWithContext is a mock that expects context as the first parameter
type MockGenWithContext struct {
	mock.Mock
}

func (m *MockGenWithContext) GenerateContent(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (string, error) {
	mockArgs := m.Called(ctx, prompt, debug, args)
	return mockArgs.String(0), mockArgs.Error(1)
}

func (m *MockGenWithContext) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	mockArgs := m.Called(ctx, prompt, debug, attrs)
	return mockArgs.String(0), mockArgs.Error(1)
}

func TestGenInterfaceWithContext(t *testing.T) {
	// This test verifies that the Gen interface accepts context.Context
	mockGen := new(MockGenWithContext)
	ctx := context.Background()
	testPrompt := ai.Prompt{Text: "test"}
	
	mockGen.On("GenerateContent", ctx, testPrompt, true, mock.Anything).Return("response", nil)
	
	// This should compile when the interface is updated
	var gen ai.Gen = mockGen
	result, err := gen.GenerateContent(ctx, testPrompt, true)
	
	assert.NoError(t, err)
	assert.Equal(t, "response", result)
	mockGen.AssertExpectations(t)
}

func TestGenInterfaceWithContextCancellation(t *testing.T) {
	mockGen := new(MockGenWithContext)
	ctx, cancel := context.WithCancel(context.Background())
	testPrompt := ai.Prompt{Text: "test"}
	
	// Cancel the context immediately
	cancel()
	
	// Mock should receive the cancelled context
	mockGen.On("GenerateContent", ctx, testPrompt, false, mock.Anything).Return("", context.Canceled)
	
	var gen ai.Gen = mockGen
	_, err := gen.GenerateContent(ctx, testPrompt, false)
	
	assert.ErrorIs(t, err, context.Canceled)
	mockGen.AssertExpectations(t)
}