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

func (m *MockGen) CountTokens(ctx context.Context, p ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	mockArgs := m.Called(ctx, p, debug, args)
	if mockArgs.Error(1) != nil {
		return nil, mockArgs.Error(1)
	}
	return mockArgs.Get(0).(*ai.TokenCount), nil
}

func (m *MockGen) GetStatus() *ai.Status {
	mockArgs := m.Called()
	return &ai.Status{
		Connected: mockArgs.Bool(0), 
		Backend: mockArgs.String(1), 
		Message: mockArgs.String(2),
	}
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