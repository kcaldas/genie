package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfirmation(t *testing.T) {
	title := "Bash Command"
	message := "ls -la"
	executionID := "test-123"
	width := 80

	model := NewConfirmation(title, message, executionID, width)

	assert.Equal(t, title, model.title)
	assert.Equal(t, message, model.message)
	assert.Equal(t, executionID, model.executionID)
	assert.Equal(t, 0, model.selectedIndex) // Should default to "Yes"
	assert.Equal(t, width, model.width)
}

func TestConfirmationComponent_Init(t *testing.T) {
	model := NewConfirmation("Bash Command", "ls -la", "exec-1", 80)
	cmd := model.Init()
	assert.Nil(t, cmd, "Init should return nil command")
}

func TestConfirmationComponent_Navigation(t *testing.T) {
	model := NewConfirmation("Test Tool", "test command", "exec-1", 80)

	// Test up arrow (should select Yes=0)
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	newModelInterface, cmd := model.Update(upMsg)
	newModel := newModelInterface.(ConfirmationModel)
	assert.Equal(t, 0, newModel.selectedIndex)
	assert.Nil(t, cmd)

	// Test down arrow (should select No=1)
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	newModelInterface, cmd = model.Update(downMsg)
	newModel = newModelInterface.(ConfirmationModel)
	assert.Equal(t, 1, newModel.selectedIndex)
	assert.Nil(t, cmd)

	// Test up arrow from No back to Yes
	newModel.selectedIndex = 1
	newModelInterface, cmd = newModel.Update(upMsg)
	newModel = newModelInterface.(ConfirmationModel)
	assert.Equal(t, 0, newModel.selectedIndex)
	assert.Nil(t, cmd)

	// Test alternative navigation (vi-style, but not documented)
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel.selectedIndex = 1
	newModelInterface, cmd = newModel.Update(kMsg)
	newModel = newModelInterface.(ConfirmationModel)
	assert.Equal(t, 0, newModel.selectedIndex)
	assert.Nil(t, cmd)

	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModelInterface, cmd = newModel.Update(jMsg)
	newModel = newModelInterface.(ConfirmationModel)
	assert.Equal(t, 1, newModel.selectedIndex)
	assert.Nil(t, cmd)
}

func TestConfirmationComponent_DirectSelection(t *testing.T) {
	model := NewConfirmation("Test Tool", "test command", "exec-123", 80)

	testCases := []struct {
		name        string
		key         string
		expectedYes bool
	}{
		{"Key 1 selects Yes", "1", true},
		{"Key 2 selects No", "2", false},
		{"ESC selects No", "esc", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
			if tc.key == "esc" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
			}

			newModelInterface, cmd := model.Update(keyMsg)
			newModel := newModelInterface.(ConfirmationModel)
			require.NotNil(t, cmd, "Should return a command")

			// Execute the command to get the message
			msg := cmd()
			responseMsg, ok := msg.(confirmationResponseMsg)
			require.True(t, ok, "Should return confirmationResponseMsg")

			assert.Equal(t, "exec-123", responseMsg.executionID)
			assert.Equal(t, tc.expectedYes, responseMsg.confirmed)

			// Model should remain unchanged
			assert.Equal(t, model.selectedIndex, newModel.selectedIndex)
		})
	}
}

func TestConfirmationComponent_EnterConfirmsSelection(t *testing.T) {
	model := NewConfirmation("Test Tool", "test command", "exec-456", 80)

	testCases := []struct {
		name           string
		selectedIndex  int
		expectedResult bool
	}{
		{"Enter on Yes (index 0)", 0, true},
		{"Enter on No (index 1)", 1, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model.selectedIndex = tc.selectedIndex

			enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
			newModelInterface, cmd := model.Update(enterMsg)
			newModel := newModelInterface.(ConfirmationModel)
			require.NotNil(t, cmd, "Should return a command")

			// Execute the command to get the message
			msg := cmd()
			responseMsg, ok := msg.(confirmationResponseMsg)
			require.True(t, ok, "Should return confirmationResponseMsg")

			assert.Equal(t, "exec-456", responseMsg.executionID)
			assert.Equal(t, tc.expectedResult, responseMsg.confirmed)

			// Model should remain unchanged
			assert.Equal(t, tc.selectedIndex, newModel.selectedIndex)
		})
	}
}

func TestConfirmationComponent_IgnoresOtherKeys(t *testing.T) {
	model := NewConfirmation("Test Tool", "test command", "exec-1", 80)
	originalModel := model

	ignoredKeys := []string{"a", "z", "space", "tab", "ctrl+c"}
	
	for _, key := range ignoredKeys {
		t.Run("Ignores "+key, func(t *testing.T) {
			var keyMsg tea.KeyMsg
			switch key {
			case "space":
				keyMsg = tea.KeyMsg{Type: tea.KeySpace}
			case "tab":
				keyMsg = tea.KeyMsg{Type: tea.KeyTab}
			case "ctrl+c":
				keyMsg = tea.KeyMsg{Type: tea.KeyCtrlC}
			default:
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
			}

			newModelInterface, cmd := model.Update(keyMsg)
			newModel := newModelInterface.(ConfirmationModel)
			assert.Nil(t, cmd, "Should not return a command for ignored keys")
			assert.Equal(t, originalModel.selectedIndex, newModel.selectedIndex, "Selection should not change")
		})
	}
}

func TestConfirmationComponent_Rendering(t *testing.T) {
	model := NewConfirmation("Bash Command", "ls -la", "exec-1", 80)

	// Test rendering with Yes selected (default)
	view := model.View()
	assert.Contains(t, view, "Bash Command")
	assert.Contains(t, view, "ls -la")
	assert.Contains(t, view, "1. Yes")
	assert.Contains(t, view, "2. No")
	assert.Contains(t, view, "Use ↑/↓ or 1/2")

	// Test rendering with No selected
	model.selectedIndex = 1
	view = model.View()
	assert.Contains(t, view, "Bash Command")
	assert.Contains(t, view, "ls -la")
	assert.Contains(t, view, "1. Yes")
	assert.Contains(t, view, "2. No")
}

func TestConfirmationComponent_RenderingHighlight(t *testing.T) {
	model := NewConfirmation("Test Tool", "test command", "exec-1", 80)

	// Test Yes selected (index 0) - should have different styling
	model.selectedIndex = 0
	yesSelectedView := model.View()

	// Test No selected (index 1)
	model.selectedIndex = 1
	noSelectedView := model.View()

	// Views should be different when different options are selected
	assert.NotEqual(t, yesSelectedView, noSelectedView, "Views should differ based on selection")

	// Both should contain the basic elements
	for _, view := range []string{yesSelectedView, noSelectedView} {
		assert.Contains(t, view, "Test Tool")
		assert.Contains(t, view, "test command")
		assert.Contains(t, view, "1. Yes")
		assert.Contains(t, view, "2. No")
	}
}

func TestConfirmationComponent_WidthHandling(t *testing.T) {
	model := NewConfirmation("Test Tool", "test cmd", "exec-1", 20) // Very narrow width
	view := model.View()
	
	// Should still render without crashing
	assert.Contains(t, view, "Test Tool")
	assert.Contains(t, view, "test cmd")
	assert.Contains(t, view, "1. Yes")
	assert.Contains(t, view, "2. No")

	// Test with very wide width
	model.width = 200
	view = model.View()
	assert.Contains(t, view, "Test Tool")
}

func TestConfirmationComponent_MessageHandling(t *testing.T) {
	testCases := []struct {
		name    string
		message string
	}{
		{"Short message", "Continue?"},
		{"Long message", "This is a very long confirmation message"},
		{"Empty message", ""},
		{"Message with newlines", "Line 1\nLine 2\nLine 3"},
		{"Message with special chars", "Run command: rm -rf / && echo 'done'?"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := NewConfirmation("Test Tool", tc.message, "exec-1", 80)
			view := model.View()
			
			// Should contain the message (or handle empty gracefully)
			// Note: For complex messages with newlines, lipgloss styling may format them differently
			if tc.message != "" && !strings.Contains(tc.message, "\n") {
				assert.Contains(t, view, tc.message)
			} else if strings.Contains(tc.message, "\n") {
				// For multiline messages, check individual lines
				lines := strings.Split(tc.message, "\n")
				for _, line := range lines {
					if line != "" {
						assert.Contains(t, view, line)
					}
				}
			}
			
			// Should always contain the options
			assert.Contains(t, view, "1. Yes")
			assert.Contains(t, view, "2. No")
		})
	}
}

// Integration helper test - verifies the component can be used in a basic tea.Program
func TestConfirmationComponent_TeaIntegration(t *testing.T) {
	model := NewConfirmation("Test Tool", "integration test", "exec-1", 80)
	
	// Verify it satisfies tea.Model interface
	var _ tea.Model = model
	
	// Test a full interaction cycle
	// 1. Init
	cmd := model.Init()
	assert.Nil(t, cmd)
	
	// 2. Navigate down
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	modelInterface, cmd := model.Update(downMsg)
	model = modelInterface.(ConfirmationModel)
	assert.Equal(t, 1, model.selectedIndex)
	assert.Nil(t, cmd)
	
	// 3. Press enter
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	modelInterface, cmd = model.Update(enterMsg)
	model = modelInterface.(ConfirmationModel)
	require.NotNil(t, cmd)
	
	// 4. Execute command
	msg := cmd()
	responseMsg, ok := msg.(confirmationResponseMsg)
	require.True(t, ok)
	assert.Equal(t, "exec-1", responseMsg.executionID)
	assert.False(t, responseMsg.confirmed) // Selected No
	
	// 5. Render
	view := model.View()
	assert.NotEmpty(t, view)
}