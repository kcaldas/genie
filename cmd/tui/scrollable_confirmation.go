package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kcaldas/genie/pkg/events"
)

// scrollableConfirmationResponseMsg is sent when user makes a confirmation choice
type scrollableConfirmationResponseMsg struct {
	executionID string
	confirmed   bool
}

// ScrollableConfirmationModel represents a scrollable confirmation dialog for content display
type ScrollableConfirmationModel struct {
	title         string
	filePath      string      // For diffs: file path, for plans: empty
	diffContent   string      // Content to display (diff or plan)
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
	scrollableConfirmationStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F59E0B")).
		Padding(1, 2)

	scrollableTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	scrollableFilePathStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6")).
		Italic(true)

	scrollableContainerStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	// Content syntax highlighting styles
	scrollableAddedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22C55E")) // Green for additions

	scrollableRemovedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")) // Red for deletions

	scrollableContextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")) // Gray for context

	scrollableHeaderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6")) // Blue for headers

	scrollableOptionStyle = lipgloss.NewStyle()

	scrollableSelectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	scrollableHelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")) // Light gray color
)

// NewScrollableConfirmation creates a new scrollable confirmation dialog from a UserConfirmationRequest
func NewScrollableConfirmation(request events.UserConfirmationRequest, width, height int) ScrollableConfirmationModel {
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

	return ScrollableConfirmationModel{
		title:         request.Title,
		filePath:      request.FilePath,
		diffContent:   request.Content,
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

// NewDiffConfirmation creates a new diff confirmation dialog (deprecated, use NewScrollableConfirmation)
func NewDiffConfirmation(title, filePath, diffContent, executionID string, width, height int) ScrollableConfirmationModel {
	request := events.UserConfirmationRequest{
		ExecutionID: executionID,
		Title:       title,
		Content:     diffContent,
		ContentType: "diff",
		FilePath:    filePath,
	}
	return NewScrollableConfirmation(request, width, height)
}

// NewPlanConfirmation creates a new plan confirmation dialog (deprecated, use NewScrollableConfirmation)
func NewPlanConfirmation(title, planContent, executionID string, width, height int) ScrollableConfirmationModel {
	request := events.UserConfirmationRequest{
		ExecutionID: executionID,
		Title:       title,
		Content:     planContent,
		ContentType: "plan",
	}
	return NewScrollableConfirmation(request, width, height)
}

// Init initializes the scrollable confirmation dialog (required by tea.Model interface)
func (m ScrollableConfirmationModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the scrollable confirmation dialog
func (m ScrollableConfirmationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedIndex == 1 {
				// Navigate from No to Yes
				m.selectedIndex = 0
			} else {
				// Scroll up in diff content if possible
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
				// Scroll down in diff content if possible
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
			// Page up in diff content
			m.scrollOffset -= 5
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			return m, nil
		case "pgdown", "ctrl+d":
			// Page down in diff content
			m.scrollOffset += 5
			if m.scrollOffset > m.maxScroll {
				m.scrollOffset = m.maxScroll
			}
			return m, nil
		case "1":
			// Direct selection: Yes
			return m, func() tea.Msg {
				return scrollableConfirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   true,
				}
			}
		case "2", "esc":
			// Direct selection: No
			return m, func() tea.Msg {
				return scrollableConfirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   false,
				}
			}
		case "enter":
			// Confirm current selection
			confirmed := m.selectedIndex == 0 // Yes=0, No=1
			return m, func() tea.Msg {
				return scrollableConfirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   confirmed,
				}
			}
		}
	}
	return m, nil
}

// View renders the scrollable confirmation dialog
func (m ScrollableConfirmationModel) View() string {
	// Prepare option rendering using custom text
	yesText := "Yes - " + m.confirmText
	noText := "No  - " + m.cancelText + " "
	
	var yesOption, noOption string
	if m.selectedIndex == 0 {
		// Yes is selected
		yesOption = scrollableSelectedStyle.Render("▶ 1. " + yesText)
		noOption = scrollableOptionStyle.Render("  2. " + noText) + scrollableHelpStyle.Render("(or Esc)")
	} else {
		// No is selected
		yesOption = scrollableOptionStyle.Render("  1. " + yesText)
		noOption = scrollableSelectedStyle.Render("▶ 2. " + noText) + scrollableHelpStyle.Render("(or Esc)")
	}

	// Render content with appropriate styling
	styledContent := m.renderStyledContent()

	// Build the complete dialog
	title := scrollableTitleStyle.Render(m.title)
	
	// Content label based on type
	contentLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render("Changes to be made:")
	
	if m.contentType == "plan" {
		contentLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render("Implementation Plan:")
	}

	// Create help text with scroll indicators
	helpText := scrollableHelpStyle.Render("Use ↑/↓ or 1/2 to select, Enter to confirm")
	if m.maxScroll > 0 {
		scrollInfo := fmt.Sprintf("(PgUp/PgDn to scroll diff, showing %d-%d of %d lines)", 
			m.scrollOffset+1, 
			min(m.scrollOffset+m.getContentDisplayHeight(), len(strings.Split(m.diffContent, "\n"))),
			len(strings.Split(m.diffContent, "\n")))
		helpText += "\n" + scrollableHelpStyle.Render(scrollInfo)
	}

	// Build content differently based on type
	var content string
	if m.contentType == "plan" {
		content = fmt.Sprintf("%s\n\n%s\n%s\n\n%s\n%s\n\n%s", 
			title, contentLabel, styledContent, yesOption, noOption, helpText)
	} else {
		// For diffs, include file path
		filePath := scrollableFilePathStyle.Render(m.filePath)
		content = fmt.Sprintf("%s\n%s\n\n%s\n%s\n\n%s\n%s\n\n%s", 
			title, filePath, contentLabel, styledContent, yesOption, noOption, helpText)
	}

	// Apply styling and return
	dialogWidth := m.width - 6 // Account for padding and borders
	if dialogWidth < 60 {
		dialogWidth = 60 // Minimum width for diff display
	}

	return scrollableConfirmationStyle.Width(dialogWidth).Render(content)
}

// renderStyledContent applies syntax highlighting to the content
func (m ScrollableConfirmationModel) renderStyledContent() string {
	if m.diffContent == "" {
		if m.contentType == "plan" {
			return scrollableContextStyle.Render("No plan to display")
		}
		return scrollableContextStyle.Render("No changes to display")
	}

	lines := strings.Split(m.diffContent, "\n")
	
	// Calculate visible lines based on scroll offset
	displayHeight := m.getContentDisplayHeight()
	startLine := m.scrollOffset
	endLine := min(startLine+displayHeight, len(lines))
	
	var styledLines []string
	for i := startLine; i < endLine; i++ {
		line := lines[i]
		if line == "" {
			styledLines = append(styledLines, "")
			continue
		}

		var styledLine string
		if m.contentType == "plan" {
			// For plans, apply basic styling - headers in blue, regular text in default
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "##") {
				styledLine = scrollableHeaderStyle.Render(line)
			} else if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
				styledLine = line // Keep bullet points as-is
			} else {
				styledLine = line
			}
		} else {
			// For diffs, apply diff syntax highlighting
			switch {
			case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
				styledLine = scrollableHeaderStyle.Render(line)
			case strings.HasPrefix(line, "@@"):
				styledLine = scrollableHeaderStyle.Render(line)
			case strings.HasPrefix(line, "+"):
				styledLine = scrollableAddedStyle.Render(line)
			case strings.HasPrefix(line, "-"):
				styledLine = scrollableRemovedStyle.Render(line)
			default:
				styledLine = scrollableContextStyle.Render(line)
			}
		}
		styledLines = append(styledLines, styledLine)
	}

	diffContent := strings.Join(styledLines, "\n")
	
	// Calculate container width for the diff
	containerWidth := m.width - 12 // Account for dialog padding, borders, and diff container borders
	if containerWidth < 40 {
		containerWidth = 40
	}

	return scrollableContainerStyle.Width(containerWidth).Render(diffContent)
}

// getContentDisplayHeight calculates how many lines of content to show based on dialog height
func (m ScrollableConfirmationModel) getContentDisplayHeight() int {
	// Reserve space for:
	// - Title (1 line)
	// - File path (1 line) 
	// - "Changes to be made:" (1 line)
	// - Options (2 lines)
	// - Help text (2 lines)
	// - Spacing and borders (4 lines)
	reservedLines := 11
	
	availableHeight := m.height - reservedLines
	if availableHeight < 5 {
		availableHeight = 5 // Minimum diff display height
	}
	
	return availableHeight
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}