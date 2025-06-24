package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ContextViewModel represents the context view modal state
type ContextViewModel struct {
	viewport viewport.Model
	content  string
	width    int
	height   int
}

// NewContextView creates a new context view component
func NewContextView(content string, width, height int) ContextViewModel {
	// Create viewport for the context content
	vp := viewport.New(width-6, height-6) // Leave margin for border and instructions
	vp.SetContent(content)
	
	return ContextViewModel{
		viewport: vp,
		content:  content,
		width:    width,
		height:   height,
	}
}

// Update handles messages for the context view
func (m ContextViewModel) Update(msg tea.Msg) (ContextViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			// Signal to close the modal - parent will handle this
			return m, func() tea.Msg { return closeContextViewMsg{} }
		case "pgup", "pgdown", "up", "down", "home", "end":
			// Handle viewport navigation
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		// Handle window resize
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 6
		m.viewport.Height = msg.Height - 6
		return m, nil
	}
	
	return m, nil
}

// View renders the context view modal
func (m ContextViewModel) View() string {
	if m.content == "" {
		content := "Context is empty"
		return m.renderModal(content)
	}
	
	// Style the context content with lighter gray for better readability
	contextStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")) // Lighter gray for context text
	
	// Apply styling to the content
	styledContent := contextStyle.Render(m.content)
	
	// Update viewport with styled content
	m.viewport.SetContent(styledContent)
	content := m.viewport.View()
	return m.renderModal(content)
}

// renderModal renders the modal with border and instructions
func (m ContextViewModel) renderModal(content string) string {
	// Modal styling
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 2).
		Margin(1, 2)
	
	// Header with title
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Render("Context View")
	
	// Instructions at bottom
	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true).
		Render("Use ↑/↓ or PgUp/PgDn to scroll • Press ESC or Q to close")
	
	// Combine header, content, and instructions
	modalContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		content,
		"",
		instructions,
	)
	
	return modalStyle.Render(modalContent)
}

// closeContextViewMsg is sent when the context view should be closed
type closeContextViewMsg struct{}