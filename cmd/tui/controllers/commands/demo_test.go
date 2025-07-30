package commands

import (
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/events"
)

func TestDemoCommand_Basic(t *testing.T) {
	// Create a mock event bus and notification
	mockEventBus := events.NewEventBus()
	mockNotification := &types.MockNotification{}
	
	// Create demo command
	cmd := NewDemoCommand(mockEventBus, mockNotification)
	
	// Test basic properties
	if cmd.GetName() != "demo" {
		t.Errorf("Expected name 'demo', got '%s'", cmd.GetName())
	}
	
	if !cmd.IsHidden() {
		t.Error("Demo command should be hidden")
	}
	
	if cmd.GetCategory() != "Development" {
		t.Errorf("Expected category 'Development', got '%s'", cmd.GetCategory())
	}
}

func TestDemoCommand_Execute_InvalidArgs(t *testing.T) {
	mockEventBus := events.NewEventBus()
	mockNotification := &types.MockNotification{}
	cmd := NewDemoCommand(mockEventBus, mockNotification)
	
	// Test with no arguments
	err := cmd.Execute([]string{})
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}
	
	// Test with invalid argument
	err = cmd.Execute([]string{"invalid"})
	if err == nil {
		t.Error("Expected error when invalid argument provided")
	}
}

func TestDemoCommand_Execute_ValidArgs(t *testing.T) {
	mockEventBus := events.NewEventBus()
	mockNotification := &types.MockNotification{}
	cmd := NewDemoCommand(mockEventBus, mockNotification)
	
	// Test diff demo
	err := cmd.Execute([]string{"diff"})
	if err != nil {
		t.Errorf("Expected no error for 'diff' argument, got: %v", err)
	}
	
	// Test markdown demo
	err = cmd.Execute([]string{"markdown"})
	if err != nil {
		t.Errorf("Expected no error for 'markdown' argument, got: %v", err)
	}
	
	// Test chat demo
	err = cmd.Execute([]string{"chat"})
	if err != nil {
		t.Errorf("Expected no error for 'chat' argument, got: %v", err)
	}
	
	// Verify that assistant messages were added via notification
	if len(mockNotification.AssistantMessages) == 0 {
		t.Error("Expected assistant messages to be added for chat demo")
	}
}