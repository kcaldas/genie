// Package confirmation provides a confirmation dialog component for Genie TUI.
// It follows the Bubble Tea component patterns for consistent behavior.
package confirmation

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kcaldas/genie/cmd/tui/theme"
)

// ResponseMsg is sent when user makes a confirmation choice
type ResponseMsg struct {
	ExecutionID string
	Confirmed   bool
}

// Topic returns the event topic for this message
func (r ResponseMsg) Topic() string {
	return "confirmation.response"
}

// Model represents a confirmation dialog following Bubbles patterns
type Model struct {
	title         string
	message       string
	executionID   string
	selectedIndex int // 0=Yes, 1=No
	width         int
}


// New creates a new confirmation dialog following Bubbles patterns
func New(title, message, executionID string, width int) Model {
	return Model{
		title:         title,
		message:       message,
		executionID:   executionID,
		selectedIndex: 0, // Default to "Yes"
		width:         width,
	}
}

// Init initializes the confirmation dialog (required by tea.Model interface)
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the confirmation dialog
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			// Navigate to Yes (0)
			m.selectedIndex = 0
			return m, nil
		case "down", "j":
			// Navigate to No (1)
			m.selectedIndex = 1
			return m, nil
		case "1":
			// Direct selection: Yes
			return m, func() tea.Msg {
				return ResponseMsg{
					ExecutionID: m.executionID,
					Confirmed:   true,
				}
			}
		case "2", "esc":
			// Direct selection: No
			return m, func() tea.Msg {
				return ResponseMsg{
					ExecutionID: m.executionID,
					Confirmed:   false,
				}
			}
		case "enter":
			// Confirm current selection
			confirmed := m.selectedIndex == 0 // Yes=0, No=1
			return m, func() tea.Msg {
				return ResponseMsg{
					ExecutionID: m.executionID,
					Confirmed:   confirmed,
				}
			}
		}
	}
	return m, nil
}

// View renders the confirmation dialog using current theme
func (m Model) View() string {
	// Get current theme styles
	styles := theme.GetStyles()
	
	// Prepare option rendering as a proper list
	var yesOption, noOption string
	
	if m.selectedIndex == 0 {
		// Yes is selected - show arrow indicator
		yesOption = styles.ConfirmationSelected.Render("▶ 1. Yes")
		noOption = styles.ConfirmationOption.Render("  2. No ") + styles.ConfirmationHelp.Render("(or Esc)")
	} else {
		// No is selected - show arrow indicator  
		yesOption = styles.ConfirmationOption.Render("  1. Yes")
		noOption = styles.ConfirmationSelected.Render("▶ 2. No ") + styles.ConfirmationHelp.Render("(or Esc)")
	}
	
	// Create the dialog content with title and message
	title := styles.ConfirmationTitle.Render(m.title)
	message := styles.ConfirmationMessage.Render(m.message)
	helpText := styles.ConfirmationHelp.Render("Use ↑/↓ or 1/2 to select, Enter to confirm")
	content := fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\n%s", 
		title, message, yesOption, noOption, helpText)
	
	// Apply styling and return
	dialogWidth := m.width - 6 // Account for padding and borders
	if dialogWidth < 40 {
		dialogWidth = 40 // Minimum width
	}
	
	return styles.ConfirmationDialog.Width(dialogWidth).Render(content)
}

// SetSize updates the width of the confirmation dialog
func (m Model) SetSize(width int) Model {
	m.width = width
	return m
}

// GetExecutionID returns the execution ID for this confirmation
func (m Model) GetExecutionID() string {
	return m.executionID
}

// GetTitle returns the title for this confirmation
func (m Model) GetTitle() string {
	return m.title
}

// GetMessage returns the message for this confirmation
func (m Model) GetMessage() string {
	return m.message
}

// GetSelectedIndex returns the currently selected index (0=Yes, 1=No)
func (m Model) GetSelectedIndex() int {
	return m.selectedIndex
}