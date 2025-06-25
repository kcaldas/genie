package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FocusPanel represents which panel is currently focused
type FocusPanel int

const (
	FocusLeft FocusPanel = iota
	FocusRight
)

// ContextViewModel represents the dual-panel context view modal state
type ContextViewModel struct {
	contextParts  map[string]string
	keys          []string // Sorted list of context keys
	selectedKey   string   // Currently selected key
	selectedIndex int      // Index of selected key in keys slice
	focusPanel    FocusPanel
	viewport      viewport.Model // For content display
	width         int
	height        int
}

// NewContextView creates a new dual-panel context view component
func NewContextView(contextParts map[string]string, width, height int) ContextViewModel {
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
	
	return ContextViewModel{
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

// Update handles messages for the context view
func (m ContextViewModel) Update(msg tea.Msg) (ContextViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return closeContextViewMsg{} }
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
				m.updateContent()
			}
		case "down", "j":
			// Navigate keys in left panel  
			if len(m.keys) > 0 && m.selectedIndex < len(m.keys)-1 {
				m.selectedIndex++
				m.selectedKey = m.keys[m.selectedIndex]
				m.updateContent()
			}
		case "pgup", "pgdown", "ctrl+u", "ctrl+d", "home", "end":
			// Always scroll right panel content
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width - 6
		m.height = msg.Height - 4
		m.viewport.Width = m.width - 20
		m.viewport.Height = m.height - 8
		return m, nil
	}
	
	return m, nil
}

// updateContent updates the viewport content based on selected key
func (m *ContextViewModel) updateContent() {
	content := ""
	if m.selectedKey != "" {
		if value, exists := m.contextParts[m.selectedKey]; exists {
			content = value
		}
	}
	m.viewport.SetContent(content)
	m.viewport.GotoTop() // Reset scroll position when switching keys
}

// View renders the dual-panel context view modal
func (m ContextViewModel) View() string {
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
func (m ContextViewModel) renderSimpleLeftPanel() string {
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
func (m ContextViewModel) renderSimpleRightPanel() string {
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
func (m ContextViewModel) renderSimpleModal(content string) string {
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

// closeContextViewMsg is sent when the context view should be closed
type closeContextViewMsg struct{}