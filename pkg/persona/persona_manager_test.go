package persona

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
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

// MockConfigManager is a mock implementation of config.Manager
type MockConfigManager struct {
	mock.Mock
}

func (m *MockConfigManager) GetString(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}

func (m *MockConfigManager) GetStringWithDefault(key, defaultValue string) string {
	args := m.Called(key, defaultValue)
	return args.String(0)
}

func (m *MockConfigManager) RequireString(key string) string {
	args := m.Called(key)
	return args.String(0)
}

func (m *MockConfigManager) GetInt(key string) (int, error) {
	args := m.Called(key)
	return args.Int(0), args.Error(1)
}

func (m *MockConfigManager) GetIntWithDefault(key string, defaultValue int) int {
	args := m.Called(key, defaultValue)
	return args.Int(0)
}

func (m *MockConfigManager) GetBoolWithDefault(key string, defaultValue bool) bool {
	args := m.Called(key, defaultValue)
	return args.Bool(0)
}

func (m *MockConfigManager) GetModelConfig() config.ModelConfig {
	args := m.Called()
	return args.Get(0).(config.ModelConfig)
}

func (m *MockConfigManager) GetDurationWithDefault(key string, defaultValue time.Duration) time.Duration {
	args := m.Called(key, defaultValue)
	return args.Get(0).(time.Duration)
}

// TestDefaultPersonaManager_GetPrompt tests the default implementation
func TestDefaultPersonaManager_GetPrompt(t *testing.T) {
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	
	// Set up config expectations - should return "genie" as default
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := NewDefaultPersonaManager(mockFactory, mockConfig)

	ctx := context.Background()

	// Create a mock prompt
	mockPrompt := &ai.Prompt{}

	// Set up expectations - default persona is "genie"
	mockFactory.On("GetPrompt", ctx, "genie").Return(mockPrompt, nil)

	// Call the method
	prompt, err := manager.GetPrompt(ctx)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, prompt)
	assert.Equal(t, mockPrompt, prompt)

	// Verify expectations were met
	mockFactory.AssertExpectations(t)
	mockConfig.AssertExpectations(t)
}

// TestDefaultPersonaManager_GetPrompt_FactoryError tests error handling from factory
func TestDefaultPersonaManager_GetPrompt_FactoryError(t *testing.T) {
	mockFactory := new(MockPersonaAwarePromptFactory)
	mockConfig := new(MockConfigManager)
	
	// Set up config expectations - should return "genie" as default
	mockConfig.On("GetStringWithDefault", "GENIE_PERSONA", "genie").Return("genie")
	
	manager := NewDefaultPersonaManager(mockFactory, mockConfig)

	ctx := context.Background()

	// Set up expectations for error case - default persona is "genie"
	mockFactory.On("GetPrompt", ctx, "genie").Return(nil, assert.AnError)

	// Call the method
	prompt, err := manager.GetPrompt(ctx)

	// Assert error results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "assert.AnError general error for testing")
	assert.Nil(t, prompt)

	// Verify expectations were met
	mockFactory.AssertExpectations(t)
	mockConfig.AssertExpectations(t)
}