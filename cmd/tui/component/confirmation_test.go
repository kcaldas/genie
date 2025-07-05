package component

import (
	"testing"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// mockConfirmationGuiCommon for testing
type mockConfirmationGuiCommon struct{}

func (m *mockConfirmationGuiCommon) GetGui() *gocui.Gui { return nil }
func (m *mockConfirmationGuiCommon) GetConfig() *types.Config { return &types.Config{} }
func (m *mockConfirmationGuiCommon) GetTheme() *types.Theme { return &types.Theme{} }
func (m *mockConfirmationGuiCommon) SetCurrentComponent(ctx types.Component) {}
func (m *mockConfirmationGuiCommon) GetCurrentComponent() types.Component { return nil }
func (m *mockConfirmationGuiCommon) PostUIUpdate(fn func()) { fn() }

func TestConfirmationComponent_Creation(t *testing.T) {
	executionID := "test-123"
	message := "1 - Yes [2 - No (Esc)]"
	
	component := NewConfirmationComponent(
		&mockConfirmationGuiCommon{},
		executionID,
		message,
		func(id string, confirmed bool) error {
			return nil
		},
	)
	
	// Test initial state
	if component.ExecutionID != executionID {
		t.Errorf("Expected ExecutionID %s, got %s", executionID, component.ExecutionID)
	}
	
	if component.GetViewName() != "input" {
		t.Errorf("Expected view name 'input', got '%s'", component.GetViewName())
	}
	
	// Test title was set
	expectedTitle := " " + message + " "
	if component.GetTitle() != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, component.GetTitle())
	}
	
	// Test window properties
	props := component.GetWindowProperties()
	if props.Editable {
		t.Error("Confirmation component should not be editable")
	}
	if !props.Focusable {
		t.Error("Confirmation component should be focusable")
	}
}

func TestConfirmationComponent_Keybindings(t *testing.T) {
	component := NewConfirmationComponent(
		&mockConfirmationGuiCommon{},
		"test-123",
		"1 - Yes [2 - No (Esc)]",
		func(id string, confirmed bool) error {
			return nil
		},
	)
	
	bindings := component.GetKeybindings()
	
	// Should have 7 keybindings (1, y, Y, 2, n, N, Esc)
	if len(bindings) != 7 {
		t.Errorf("Expected 7 keybindings, got %d", len(bindings))
	}
	
	// Check for specific keys
	foundKeys := make(map[interface{}]bool)
	for _, binding := range bindings {
		foundKeys[binding.Key] = true
		
		// All bindings should be for the "input" view
		if binding.View != "input" {
			t.Errorf("Expected binding for 'input' view, got '%s'", binding.View)
		}
	}
	
	// Check Yes keys
	if !foundKeys['1'] {
		t.Error("Should have binding for '1' key")
	}
	if !foundKeys['y'] {
		t.Error("Should have binding for 'y' key")
	}
	if !foundKeys['Y'] {
		t.Error("Should have binding for 'Y' key")
	}
	
	// Check No keys
	if !foundKeys['2'] {
		t.Error("Should have binding for '2' key")
	}
	if !foundKeys['n'] {
		t.Error("Should have binding for 'n' key")
	}
	if !foundKeys['N'] {
		t.Error("Should have binding for 'N' key")
	}
	if !foundKeys[gocui.KeyEsc] {
		t.Error("Should have binding for Esc key")
	}
}

func TestConfirmationComponent_Handlers(t *testing.T) {
	var handlerCalled bool
	var handlerExecutionID string
	var handlerConfirmed bool
	
	component := NewConfirmationComponent(
		&mockConfirmationGuiCommon{},
		"test-456",
		"1 - Yes [2 - No (Esc)]",
		func(id string, confirmed bool) error {
			handlerCalled = true
			handlerExecutionID = id
			handlerConfirmed = confirmed
			return nil
		},
	)
	
	// Test "Yes" handler
	handlerCalled = false
	err := component.handleConfirmYes(nil, nil)
	if err != nil {
		t.Errorf("handleConfirmYes returned error: %v", err)
	}
	
	if !handlerCalled {
		t.Error("Handler should have been called for Yes")
	}
	if handlerExecutionID != "test-456" {
		t.Errorf("Expected executionID 'test-456', got '%s'", handlerExecutionID)
	}
	if !handlerConfirmed {
		t.Error("Handler should have received confirmed=true for Yes")
	}
	
	// Test "No" handler
	handlerCalled = false
	err = component.handleConfirmNo(nil, nil)
	if err != nil {
		t.Errorf("handleConfirmNo returned error: %v", err)
	}
	
	if !handlerCalled {
		t.Error("Handler should have been called for No")
	}
	if handlerConfirmed {
		t.Error("Handler should have received confirmed=false for No")
	}
}