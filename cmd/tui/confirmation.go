package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirmationResponseMsg is sent when user makes a confirmation choice
type confirmationResponseMsg struct {
	executionID string
	confirmed   bool
}

// ConfirmationModel represents a confirmation dialog
type ConfirmationModel struct {
	title         string
	message       string
	executionID   string
	selectedIndex int // 0=Yes, 1=No
	width         int
}

// Styles for confirmation dialog
var (
	confirmationStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F59E0B")).
		Padding(1, 2)

	confirmationTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	confirmationMessageStyle = lipgloss.NewStyle().
		PaddingLeft(4)

	confirmationOptionStyle = lipgloss.NewStyle()

	confirmationSelectedStyle = lipgloss.NewStyle()
)

// NewConfirmation creates a new confirmation dialog
func NewConfirmation(title, message, executionID string, width int) ConfirmationModel {
	return ConfirmationModel{
		title:         title,
		message:       message,
		executionID:   executionID,
		selectedIndex: 0, // Default to "Yes"
		width:         width,
	}
}

// Init initializes the confirmation dialog (required by tea.Model interface)
func (m ConfirmationModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the confirmation dialog
func (m ConfirmationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				return confirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   true,
				}
			}
		case "2", "esc":
			// Direct selection: No
			return m, func() tea.Msg {
				return confirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   false,
				}
			}
		case "enter":
			// Confirm current selection
			confirmed := m.selectedIndex == 0 // Yes=0, No=1
			return m, func() tea.Msg {
				return confirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   confirmed,
				}
			}
		}
	}
	return m, nil
}

// View renders the confirmation dialog
func (m ConfirmationModel) View() string {
	// Prepare option rendering as a proper list
	var yesOption, noOption string
	
	if m.selectedIndex == 0 {
		// Yes is selected - show arrow indicator
		yesOption = confirmationSelectedStyle.Render("▶ 1. Yes")
		noOption = confirmationOptionStyle.Render("  2. No")
	} else {
		// No is selected - show arrow indicator  
		yesOption = confirmationOptionStyle.Render("  1. Yes")
		noOption = confirmationSelectedStyle.Render("▶ 2. No")
	}
	
	// Create the dialog content with title and message
	title := confirmationTitleStyle.Render(m.title)
	message := confirmationMessageStyle.Render(m.message)
	content := fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\nUse ↑/↓ or 1/2 to select, Enter to confirm", 
		title, message, yesOption, noOption)
	
	// Apply styling and return
	dialogWidth := m.width - 6 // Account for padding and borders
	if dialogWidth < 40 {
		dialogWidth = 40 // Minimum width
	}
	
	return confirmationStyle.Width(dialogWidth).Render(content)
}