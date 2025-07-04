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

func (m *MockPersonaManager) GetChain(ctx context.Context) (*ai.Chain, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ai.Chain), args.Error(1)
}

// TestPersonaManagerInterface ensures the interface is properly defined
func TestPersonaManagerInterface(t *testing.T) {
	// This test verifies that MockPersonaManager implements PersonaManager
	var _ PersonaManager = (*MockPersonaManager)(nil)
}

// TestMockPersonaManager_GetChain tests the mock implementation
func TestMockPersonaManager_GetChain(t *testing.T) {
	mockManager := new(MockPersonaManager)
	ctx := context.Background()

	// Create a mock chain
	mockChain := &ai.Chain{}

	// Set up expectations
	mockManager.On("GetChain", ctx).Return(mockChain, nil)

	// Call the method
	chain, err := mockManager.GetChain(ctx)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, chain)
	assert.Equal(t, mockChain, chain)

	// Verify expectations were met
	mockManager.AssertExpectations(t)
}

// TestMockPersonaManager_GetChain_Error tests error handling
func TestMockPersonaManager_GetChain_Error(t *testing.T) {
	mockManager := new(MockPersonaManager)
	ctx := context.Background()

	// Set up expectations for error case
	mockManager.On("GetChain", ctx).Return(nil, assert.AnError)

	// Call the method
	chain, err := mockManager.GetChain(ctx)

	// Assert error results
	assert.Error(t, err)
	assert.Nil(t, chain)

	// Verify expectations were met
	mockManager.AssertExpectations(t)
}

// MockChainFactory is a mock implementation of ChainFactory
type MockChainFactory struct {
	mock.Mock
}

func (m *MockChainFactory) CreateChain() (*ai.Chain, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ai.Chain), args.Error(1)
}

// TestDefaultPersonaManager_GetChain tests the default implementation
func TestDefaultPersonaManager_GetChain(t *testing.T) {
	mockFactory := new(MockChainFactory)
	manager := NewDefaultPersonaManager(mockFactory)

	ctx := context.Background()

	// Create a mock chain
	mockChain := &ai.Chain{}

	// Set up expectations
	mockFactory.On("CreateChain").Return(mockChain, nil)

	// Call the method
	chain, err := manager.GetChain(ctx)

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, chain)
	assert.Equal(t, mockChain, chain)

	// Verify expectations were met
	mockFactory.AssertExpectations(t)
}

// TestDefaultPersonaManager_GetChain_FactoryError tests error handling from factory
func TestDefaultPersonaManager_GetChain_FactoryError(t *testing.T) {
	mockFactory := new(MockChainFactory)
	manager := NewDefaultPersonaManager(mockFactory)

	ctx := context.Background()

	// Set up expectations for error case
	mockFactory.On("CreateChain").Return(nil, assert.AnError)

	// Call the method
	chain, err := manager.GetChain(ctx)

	// Assert error results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create chain")
	assert.Nil(t, chain)

	// Verify expectations were met
	mockFactory.AssertExpectations(t)
}

