package persona

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPersonaManager is a mock implementation of PersonaManager
type MockPersonaManager struct {
	mock.Mock
}

func (m *MockPersonaManager) GetPrompt(ctx context.Context) (*ai.Prompt, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ai.Prompt), args.Error(1)
}

// TestPersonaManagerInterface ensures the interface is properly defined
func TestPersonaManagerInterface(t *testing.T) {
	// This test verifies that MockPersonaManager implements PersonaManager
	var _ PersonaManager = (*MockPersonaManager)(nil)
}

// TestMockPersonaManager_GetPrompt tests the mock implementation
func TestMockPersonaManager_GetPrompt(t *testing.T) {
	mockManager := new(MockPersonaManager)
	ctx := context.Background()

	// Create a mock prompt
	mockPrompt := &ai.Prompt{}

	// Set up expectations
	mockManager.On("GetPrompt", ctx).Return(mockPrompt, nil)

	// Call the method
	prompt, err := mockManager.GetPrompt(ctx)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, prompt)
	assert.Equal(t, mockPrompt, prompt)

	// Verify expectations were met
	mockManager.AssertExpectations(t)
}

// TestMockPersonaManager_GetPrompt_Error tests error handling
func TestMockPersonaManager_GetPrompt_Error(t *testing.T) {
	mockManager := new(MockPersonaManager)
	ctx := context.Background()

	// Set up expectations for error case
	mockManager.On("GetPrompt", ctx).Return(nil, assert.AnError)

	// Call the method
	prompt, err := mockManager.GetPrompt(ctx)

	// Assert error results
	assert.Error(t, err)
	assert.Nil(t, prompt)

	// Verify expectations were met
	mockManager.AssertExpectations(t)
}

// MockPersonaAwarePromptFactory is a mock implementation of persona.PersonaAwarePromptFactory
type MockPersonaAwarePromptFactory struct {
	mock.Mock
}

func (m *MockPersonaAwarePromptFactory) GetPrompt(ctx context.Context, personaName string) (*ai.Prompt, error) {
	args := m.Called(ctx, personaName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ai.Prompt), args.Error(1)
}

// TestDefaultPersonaManager_GetPrompt tests the default implementation
func TestDefaultPersonaManager_GetPrompt(t *testing.T) {
	mockFactory := new(MockPersonaAwarePromptFactory)
	manager := NewDefaultPersonaManager(mockFactory)

	ctx := context.Background()

	// Create a mock prompt
	mockPrompt := &ai.Prompt{}

	// Set up expectations - default persona is "engineer"
	mockFactory.On("GetPrompt", ctx, "engineer").Return(mockPrompt, nil)

	// Call the method
	prompt, err := manager.GetPrompt(ctx)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, prompt)
	assert.Equal(t, mockPrompt, prompt)

	// Verify expectations were met
	mockFactory.AssertExpectations(t)
}

// TestDefaultPersonaManager_GetPrompt_FactoryError tests error handling from factory
func TestDefaultPersonaManager_GetPrompt_FactoryError(t *testing.T) {
	mockFactory := new(MockPersonaAwarePromptFactory)
	manager := NewDefaultPersonaManager(mockFactory)

	ctx := context.Background()

	// Set up expectations for error case - default persona is "engineer"
	mockFactory.On("GetPrompt", ctx, "engineer").Return(nil, assert.AnError)

	// Call the method
	prompt, err := manager.GetPrompt(ctx)

	// Assert error results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "assert.AnError general error for testing")
	assert.Nil(t, prompt)

	// Verify expectations were met
	mockFactory.AssertExpectations(t)
}
