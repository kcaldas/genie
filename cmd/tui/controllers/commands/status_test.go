package commands

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/stretchr/testify/assert"
)

func TestStatusCommand_Execute(t *testing.T) {
	// Create mock notification
	mockNotification := &types.MockNotification{}
	
	// Create mock genie service
	mockGenie := &MockGenieService{}
	
	// Create status command
	cmd := NewStatusCommand(mockNotification, mockGenie)
	
	// Test basic metadata
	assert.Equal(t, "status", cmd.GetName())
	assert.Equal(t, "Show the current status of the AI backend", cmd.GetDescription())
	assert.Contains(t, cmd.GetAliases(), "st")
	assert.Equal(t, "System", cmd.GetCategory())
	
	// Test execution with connected status
	mockGenie.mockStatus = &genie.Status{
		Connected: true,
		Backend:   "gemini",
		Message:   "API configured and ready",
	}
	
	err := cmd.Execute([]string{})
	assert.NoError(t, err)
	assert.Len(t, mockNotification.SystemMessages, 1)
	assert.Contains(t, mockNotification.SystemMessages[0], "✓")
	assert.Contains(t, mockNotification.SystemMessages[0], "gemini")
	assert.Contains(t, mockNotification.SystemMessages[0], "API configured and ready")
	
	// Test execution with disconnected status
	mockNotification.SystemMessages = []string{} // Reset
	mockGenie.mockStatus = &genie.Status{
		Connected: false,
		Backend:   "vertex",
		Message:   "Project ID not configured",
	}
	
	err = cmd.Execute([]string{})
	assert.NoError(t, err)
	assert.Len(t, mockNotification.SystemMessages, 1)
	assert.Contains(t, mockNotification.SystemMessages[0], "✗")
	assert.Contains(t, mockNotification.SystemMessages[0], "vertex")
	assert.Contains(t, mockNotification.SystemMessages[0], "Project ID not configured")
}

// MockGenieService for testing
type MockGenieService struct {
	mockStatus *genie.Status
}

func (m *MockGenieService) Start(workingDir *string, persona *string) (*genie.Session, error) {
	return &genie.Session{}, nil
}

func (m *MockGenieService) Chat(ctx context.Context, message string) error {
	return nil
}

func (m *MockGenieService) GetContext(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *MockGenieService) GetStatus() *genie.Status {
	return m.mockStatus
}

func (m *MockGenieService) GetEventBus() events.EventBus {
	return nil
}