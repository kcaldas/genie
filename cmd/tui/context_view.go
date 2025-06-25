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

// calculateLeftPanelWidth calculates the optimal width based on the largest key
func calculateLeftPanelWidth(keys []string) int {
	if len(keys) == 0 {
		return 15 // Minimum width for "No context parts"
	}
	
	maxKeyLength := 0
	for _, key := range keys {
		keyDisplayLength := len("> " + key) // Account for selection prefix
		if keyDisplayLength > maxKeyLength {
			maxKeyLength = keyDisplayLength
		}
	}
	
	// Add some padding (4 characters) for comfortable spacing
	return maxKeyLength + 4
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
	
	// Calculate optimal left panel width based on key lengths
	leftPanelWidth := calculateLeftPanelWidth(keys)
	// Ensure it doesn't exceed 25% of total width
	maxLeftWidth := width * 25 / 100
	if leftPanelWidth > maxLeftWidth {
		leftPanelWidth = maxLeftWidth
	}
	
	// Create viewport for content display (right panel)
	contentWidth := width - leftPanelWidth - 6 // Remaining width minus margins
	contentHeight := height - 6 // Leave space for header and footer with proper margins
	vp := viewport.New(contentWidth, contentHeight)
	
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
		focusPanel:    FocusLeft, // Start with left panel focused
		viewport:      vp,
		width:         width,
		height:        height,
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
		case "tab":
			// Switch between panels
			if m.focusPanel == FocusLeft {
				m.focusPanel = FocusRight
			} else {
				m.focusPanel = FocusLeft
			}
			return m, nil
		case "up", "k":
			if m.focusPanel == FocusLeft {
				// Navigate keys in left panel
				if m.selectedIndex > 0 {
					m.selectedIndex--
					m.selectedKey = m.keys[m.selectedIndex]
					m.updateContent()
				}
			} else {
				// Scroll content in right panel
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		case "down", "j":
			if m.focusPanel == FocusLeft {
				// Navigate keys in left panel
				if m.selectedIndex < len(m.keys)-1 {
					m.selectedIndex++
					m.selectedKey = m.keys[m.selectedIndex]
					m.updateContent()
				}
			} else {
				// Scroll content in right panel
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		case "pgup", "pgdown", "home", "end":
			if m.focusPanel == FocusRight {
				// Handle viewport navigation in right panel
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}
	case tea.WindowSizeMsg:
		// Handle window resize
		m.width = msg.Width
		m.height = msg.Height
		
		// Update viewport dimensions
		leftPanelWidth := calculateLeftPanelWidth(m.keys)
		maxLeftWidth := msg.Width * 25 / 100
		if leftPanelWidth > maxLeftWidth {
			leftPanelWidth = maxLeftWidth
		}
		contentWidth := msg.Width - leftPanelWidth - 6
		contentHeight := msg.Height - 6
		m.viewport.Width = contentWidth
		m.viewport.Height = contentHeight
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
		content := "Context is empty"
		return m.renderModal(content)
	}
	
	// Create left panel (keys)
	leftPanel := m.renderLeftPanel()
	
	// Create right panel (content)
	rightPanel := m.renderRightPanel()
	
	// Combine panels horizontally
	panelsContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		rightPanel,
	)
	
	return m.renderModal(panelsContent)
}

// renderLeftPanel renders the left panel with context keys
func (m ContextViewModel) renderLeftPanel() string {
	// Calculate optimal width based on key lengths
	panelWidth := calculateLeftPanelWidth(m.keys)
	maxLeftWidth := m.width * 25 / 100
	if panelWidth > maxLeftWidth {
		panelWidth = maxLeftWidth
	}
	panelHeight := m.height - 6
	
	// Simple panel style without border or background
	panelStyle := lipgloss.NewStyle().
		Width(panelWidth).
		Height(panelHeight).
		Padding(1)
	
	// Render key list
	var keyItems []string
	for i, key := range m.keys {
		prefix := "  "
		var style lipgloss.Style
		
		if i == m.selectedIndex {
			prefix = "> "
			if m.focusPanel == FocusLeft {
				// Bold and bright when focused and selected
				style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
			} else {
				// Less prominent when not focused
				style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D1D5DB"))
			}
		} else {
			// Regular styling for non-selected items
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
		}
		
		keyItems = append(keyItems, style.Render(prefix+key))
	}
	
	content := strings.Join(keyItems, "\n")
	if content == "" {
		content = "No context parts"
	}
	
	return panelStyle.Render(content)
}

// renderRightPanel renders the right panel with selected content
func (m ContextViewModel) renderRightPanel() string {
	leftPanelWidth := calculateLeftPanelWidth(m.keys)
	maxLeftWidth := m.width * 25 / 100
	if leftPanelWidth > maxLeftWidth {
		leftPanelWidth = maxLeftWidth
	}
	panelWidth := m.width - leftPanelWidth - 6 // Remaining width minus margins
	panelHeight := m.height - 6
	
	// Panel border style
	borderColor := "#6B7280" // Default gray
	if m.focusPanel == FocusRight {
		borderColor = "#7C3AED" // Purple when focused
	}
	
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(panelWidth).
		Height(panelHeight).
		Padding(1)
	
	// Style the content
	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))
	
	// Get content from viewport
	content := m.viewport.View()
	if content == "" {
		content = "No content for selected key"
	}
	
	return panelStyle.Render(contentStyle.Render(content))
}

// renderModal renders the modal with header and instructions
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
	focusedPanel := "Left"
	if m.focusPanel == FocusRight {
		focusedPanel = "Right"
	}
	
	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true).
		Render("Tab: Switch Panel (" + focusedPanel + ") • ↑/↓: Navigate • PgUp/PgDn: Scroll • ESC/Q: Close")
	
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