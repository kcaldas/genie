// Package scrollconfirm provides a scrollable confirmation dialog component for Genie TUI.
// It follows the Bubble Tea component patterns for consistent behavior.
package scrollconfirm

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kcaldas/genie/pkg/events"
)

// ResponseMsg is sent when user makes a confirmation choice
type ResponseMsg struct {
	ExecutionID string
	Confirmed   bool
}

// Topic returns the event topic for this message
func (r ResponseMsg) Topic() string {
	return "scrollconfirm.response"
}

// Model represents a scrollable confirmation dialog following Bubbles patterns
type Model struct {
	title         string
	filePath      string      // For diffs: file path, for plans: empty
	content       string      // Content to display (diff or plan)
	executionID   string
	selectedIndex int         // 0=Yes, 1=No
	width         int
	height        int
	scrollOffset  int
	maxScroll     int
	contentType   string      // "diff" or "plan" to determine rendering
	confirmText   string      // Custom confirm button text
	cancelText    string      // Custom cancel button text
}

// Styles for scrollable confirmation dialog
var (
	dialogStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F59E0B")).
		Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	filePathStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6")).
		Italic(true)

	containerStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	// Content syntax highlighting styles
	addedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22C55E")) // Green for additions

	removedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")) // Red for deletions

	contextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")) // Gray for context

	headerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6")) // Blue for headers

	optionStyle = lipgloss.NewStyle()

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")) // Light gray color
)

// New creates a new scrollable confirmation dialog following Bubbles patterns
func New(request events.UserConfirmationRequest, width, height int) Model {
	// Calculate max scroll based on content
	contentLines := strings.Split(request.Content, "\n")
	maxContentHeight := height - 12 // Reserve space for title, options, help text, etc.
	if maxContentHeight < 5 {
		maxContentHeight = 5
	}
	
	maxScroll := len(contentLines) - maxContentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Set default text if not provided
	confirmText := request.ConfirmText
	if confirmText == "" {
		if request.ContentType == "plan" {
			confirmText = "Proceed with implementation"
		} else {
			confirmText = "Apply changes"
		}
	}
	
	cancelText := request.CancelText
	if cancelText == "" {
		if request.ContentType == "plan" {
			cancelText = "Revise plan"
		} else {
			cancelText = "Cancel"
		}
	}

	return Model{
		title:         request.Title,
		filePath:      request.FilePath,
		content:       request.Content,
		executionID:   request.ExecutionID,
		selectedIndex: 0, // Default to "Yes"
		width:         width,
		height:        height,
		scrollOffset:  0,
		maxScroll:     maxScroll,
		contentType:   request.ContentType,
		confirmText:   confirmText,
		cancelText:    cancelText,
	}
}

// Init initializes the scrollable confirmation dialog (required by tea.Model interface)
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the scrollable confirmation dialog
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedIndex == 1 {
				// Navigate from No to Yes
				m.selectedIndex = 0
			} else {
				// Scroll up in content if possible
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			}
			return m, nil
		case "down", "j":
			if m.selectedIndex == 0 {
				// Navigate from Yes to No
				m.selectedIndex = 1
			} else {
				// Scroll down in content if possible
				if m.scrollOffset < m.maxScroll {
					m.scrollOffset++
				}
			}
			return m, nil
		case "left", "h":
			// Navigate to Yes
			m.selectedIndex = 0
			return m, nil
		case "right", "l":
			// Navigate to No
			m.selectedIndex = 1
			return m, nil
		case "pgup", "ctrl+u":
			// Page up in content
			m.scrollOffset -= 5
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			return m, nil
		case "pgdown", "ctrl+d":
			// Page down in content
			m.scrollOffset += 5
			if m.scrollOffset > m.maxScroll {
				m.scrollOffset = m.maxScroll
			}
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

// SetSize updates the dimensions of the dialog
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	
	// Recalculate max scroll
	contentLines := strings.Split(m.content, "\n")
	maxContentHeight := height - 12
	if maxContentHeight < 5 {
		maxContentHeight = 5
	}
	
	maxScroll := len(contentLines) - maxContentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.maxScroll = maxScroll
	
	// Adjust scroll offset if needed
	if m.scrollOffset > m.maxScroll {
		m.scrollOffset = m.maxScroll
	}
	
	return m
}

// View renders the scrollable confirmation dialog
func (m Model) View() string {
	// Prepare option rendering using custom text
	yesText := "Yes - " + m.confirmText
	noText := "No  - " + m.cancelText + " "
	
	var yesOption, noOption string
	if m.selectedIndex == 0 {
		// Yes is selected
		yesOption = selectedStyle.Render("▶ 1. " + yesText)
		noOption = optionStyle.Render("  2. " + noText) + helpStyle.Render("(or Esc)")
	} else {
		// No is selected
		yesOption = optionStyle.Render("  1. " + yesText)
		noOption = selectedStyle.Render("▶ 2. " + noText) + helpStyle.Render("(or Esc)")
	}
	
	// Build dialog content
	title := titleStyle.Render(m.title)
	
	var contentParts []string
	contentParts = append(contentParts, title)
	
	// Add file path if provided
	if m.filePath != "" {
		filePath := filePathStyle.Render(fmt.Sprintf("File: %s", m.filePath))
		contentParts = append(contentParts, filePath)
	}
	
	// Add scrollable content
	scrollableContent := m.renderScrollableContent()
	if scrollableContent != "" {
		contentParts = append(contentParts, scrollableContent)
	}
	
	// Add options
	contentParts = append(contentParts, yesOption)
	contentParts = append(contentParts, noOption)
	
	// Add help text
	helpText := helpStyle.Render("Use ↑/↓ to scroll, ←/→ or 1/2 to select, Enter to confirm")
	contentParts = append(contentParts, helpText)
	
	// Join all parts
	content := strings.Join(contentParts, "\n\n")
	
	// Apply styling and return
	dialogWidth := m.width - 6 // Account for padding and borders
	if dialogWidth < 50 {
		dialogWidth = 50 // Minimum width
	}
	
	return dialogStyle.Width(dialogWidth).Render(content)
}

// renderScrollableContent renders the scrollable content portion
func (m Model) renderScrollableContent() string {
	if m.content == "" {
		return ""
	}
	
	lines := strings.Split(m.content, "\n")
	maxContentHeight := m.height - 12
	if maxContentHeight < 5 {
		maxContentHeight = 5
	}
	
	// Calculate which lines to show
	startLine := m.scrollOffset
	endLine := startLine + maxContentHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}
	
	if startLine >= len(lines) {
		return ""
	}
	
	visibleLines := lines[startLine:endLine]
	
	// Apply syntax highlighting if this is a diff
	if m.contentType == "diff" {
		for i, line := range visibleLines {
			visibleLines[i] = m.highlightDiffLine(line)
		}
	}
	
	content := strings.Join(visibleLines, "\n")
	
	// Add scroll indicators if needed
	scrollInfo := ""
	if m.maxScroll > 0 {
		scrollInfo = helpStyle.Render(fmt.Sprintf("(Line %d-%d of %d)", startLine+1, endLine, len(lines)))
		content = content + "\n" + scrollInfo
	}
	
	return containerStyle.Render(content)
}

// highlightDiffLine applies syntax highlighting to diff lines
func (m Model) highlightDiffLine(line string) string {
	if len(line) == 0 {
		return line
	}
	
	switch line[0] {
	case '+':
		return addedStyle.Render(line)
	case '-':
		return removedStyle.Render(line)
	case '@':
		return headerStyle.Render(line)
	default:
		return contextStyle.Render(line)
	}
}

// GetExecutionID returns the execution ID for this confirmation
func (m Model) GetExecutionID() string {
	return m.executionID
}

// GetContentType returns the content type ("diff", "plan", etc.)
func (m Model) GetContentType() string {
	return m.contentType
}

// NewDiffConfirmation creates a new diff confirmation dialog (deprecated, use New)
func NewDiffConfirmation(title, filePath, diffContent, executionID string, width, height int) Model {
	request := events.UserConfirmationRequest{
		ExecutionID: executionID,
		Title:       title,
		Content:     diffContent,
		ContentType: "diff",
		FilePath:    filePath,
	}
	return New(request, width, height)
}

// NewPlanConfirmation creates a new plan confirmation dialog (deprecated, use New)
func NewPlanConfirmation(title, planContent, executionID string, width, height int) Model {
	request := events.UserConfirmationRequest{
		ExecutionID: executionID,
		Title:       title,
		Content:     planContent,
		ContentType: "plan",
	}
	return New(request, width, height)
}

// Test helper methods (these expose internal state for testing only)

// GetSelectedIndex returns the currently selected index for testing
func (m Model) GetSelectedIndex() int {
	return m.selectedIndex
}

// GetScrollOffset returns the current scroll offset for testing
func (m Model) GetScrollOffset() int {
	return m.scrollOffset
}

// GetMaxScroll returns the maximum scroll value for testing
func (m Model) GetMaxScroll() int {
	return m.maxScroll
}

// SetSelectedIndex sets the selected index for testing
func (m Model) SetSelectedIndex(index int) Model {
	m.selectedIndex = index
	return m
}

// SetScrollOffset sets the scroll offset for testing
func (m Model) SetScrollOffset(offset int) Model {
	m.scrollOffset = offset
	return m
}