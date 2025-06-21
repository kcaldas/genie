package ai_test

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGen is a mock that implements Gen interface
type MockGen struct {
	mock.Mock
}

func (m *MockGen) GenerateContent(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (string, error) {
	mockArgs := m.Called(ctx, prompt, debug, args)
	return mockArgs.String(0), mockArgs.Error(1)
}

func (m *MockGen) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	mockArgs := m.Called(ctx, prompt, debug, attrs)
	return mockArgs.String(0), mockArgs.Error(1)
}

func TestGenCancellation(t *testing.T) {
	mockGen := new(MockGen)
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