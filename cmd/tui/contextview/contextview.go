// Package contextview provides a dual-panel context viewer component for Genie TUI.
// It follows the Bubble Tea component patterns for consistent behavior.
package contextview

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CloseMsg is sent when the context view should be closed
type CloseMsg struct{}

// Topic returns the event topic for this message
func (c CloseMsg) Topic() string {
	return "contextview.close"
}

// FocusPanel represents which panel is currently focused
type FocusPanel int

const (
	FocusLeft FocusPanel = iota
	FocusRight
)

// Model represents the dual-panel context view modal state following Bubbles patterns
type Model struct {
	contextParts  map[string]string
	keys          []string // Sorted list of context keys
	selectedKey   string   // Currently selected key
	selectedIndex int      // Index of selected key in keys slice
	focusPanel    FocusPanel
	viewport      viewport.Model // For content display
	width         int
	height        int
}

// New creates a new dual-panel context view component following Bubbles patterns
func New(contextParts map[string]string, width, height int) Model {
	// Extract and sort keys for consistent ordering
	keys := make([]string, 0, len(contextParts))
	for key := range contextParts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	
	// Select first key if available
	selectedKey := ""
	selectedIndex := 0
	if len(keys) > 0 {
		selectedKey = keys[0]
	}
	
	// Simple approach: use most of the screen with some margin
	modalWidth := width - 6  // Small margin
	modalHeight := height - 4 // Small margin
	
	// Create a simple viewport
	vp := viewport.New(modalWidth-20, modalHeight-8) // Leave space for left panel, header, and instructions
	
	// Set initial content
	content := ""
	if selectedKey != "" {
		content = contextParts[selectedKey]
	}
	vp.SetContent(content)
	
	return Model{
		contextParts:  contextParts,
		keys:          keys,
		selectedKey:   selectedKey,
		selectedIndex: selectedIndex,
		focusPanel:    FocusLeft,
		viewport:      vp,
		width:         modalWidth,
		height:        modalHeight,
	}
}

// Init initializes the context view (required by tea.Model interface)
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the context view following Bubbles patterns
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return CloseMsg{} }
		case "tab":
			if m.focusPanel == FocusLeft {
				m.focusPanel = FocusRight
			} else {
				m.focusPanel = FocusLeft
			}
			return m, nil
		case "up", "k":
			// Navigate keys in left panel
			if len(m.keys) > 0 && m.selectedIndex > 0 {
				m.selectedIndex--
				m.selectedKey = m.keys[m.selectedIndex]
				m = m.updateContent()
			}
		case "down", "j":
			// Navigate keys in left panel  
			if len(m.keys) > 0 && m.selectedIndex < len(m.keys)-1 {
				m.selectedIndex++
				m.selectedKey = m.keys[m.selectedIndex]
				m = m.updateContent()
			}
		case "pgup", "pgdown", "ctrl+u", "ctrl+d", "home", "end":
			// Always scroll right panel content
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m = m.SetSize(msg.Width, msg.Height)
		return m, nil
	}
	
	return m, nil
}

// SetSize updates the dimensions of the context view
func (m Model) SetSize(width, height int) Model {
	m.width = width - 6
	m.height = height - 4
	m.viewport.Width = m.width - 20
	m.viewport.Height = m.height - 8
	return m
}

// updateContent updates the viewport content based on selected key
func (m Model) updateContent() Model {
	content := ""
	if m.selectedKey != "" {
		if value, exists := m.contextParts[m.selectedKey]; exists {
			content = value
		}
	}
	m.viewport.SetContent(content)
	m.viewport.GotoTop() // Reset scroll position when switching keys
	return m
}

// View renders the dual-panel context view modal
func (m Model) View() string {
	if len(m.contextParts) == 0 {
		return m.renderSimpleModal("Context is empty")
	}
	
	// Simple left panel - just list the keys
	leftPanel := m.renderSimpleLeftPanel()
	
	// Simple right panel - just the content
	rightPanel := m.renderSimpleRightPanel()
	
	// Combine panels side by side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	
	return m.renderSimpleModal(panels)
}

// renderSimpleLeftPanel renders a basic left panel with keys
func (m Model) renderSimpleLeftPanel() string {
	var keyList []string
	for i, key := range m.keys {
		if i == m.selectedIndex {
			// Selected key with arrow
			keyList = append(keyList, "> "+key)
		} else {
			// Regular key
			keyList = append(keyList, "  "+key)
		}
	}
	
	content := strings.Join(keyList, "\n")
	if content == "" {
		content = "No keys"
	}
	
	// Simple box for left panel without border
	return lipgloss.NewStyle().
		Width(18).
		Height(m.height - 6). // Account for instructions line
		Padding(1).
		Render(content)
}

// renderSimpleRightPanel renders a basic right panel with content
func (m Model) renderSimpleRightPanel() string {
	content := m.viewport.View()
	if content == "" {
		content = "No content"
	}
	
	// Simple box for right panel with darker gray border and text
	return lipgloss.NewStyle().
		Width(m.width - 22). // Total width minus left panel width
		Height(m.height - 6). // Account for instructions line
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#6B7280")). // Darker gray border
		Foreground(lipgloss.Color("#6B7280")).       // Darker gray text
		Padding(1).
		Render(content)
}

// renderSimpleModal renders a basic modal wrapper
func (m Model) renderSimpleModal(content string) string {
	// Instructions at bottom
	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true).
		Render("↑/↓: Navigate • PgUp/PgDn: Scroll • ESC/Q: Close")
	
	// Combine content and instructions
	modalContent := lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		"",
		instructions,
	)
	
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1).
		Render(modalContent)
}

// GetSelectedKey returns the currently selected key
func (m Model) GetSelectedKey() string {
	return m.selectedKey
}

// GetContentParts returns all context parts  
func (m Model) GetContentParts() map[string]string {
	return m.contextParts
}