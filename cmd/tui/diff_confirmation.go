package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// diffConfirmationResponseMsg is sent when user makes a diff confirmation choice
type diffConfirmationResponseMsg struct {
	executionID string
	confirmed   bool
}

// DiffConfirmationModel represents a confirmation dialog with diff preview
type DiffConfirmationModel struct {
	title         string
	filePath      string
	diffContent   string
	executionID   string
	selectedIndex int // 0=Yes, 1=No
	width         int
	height        int
	scrollOffset  int
	maxScroll     int
}

// Styles for diff confirmation dialog
var (
	diffConfirmationStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F59E0B")).
		Padding(1, 2)

	diffTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	diffFilePathStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6")).
		Italic(true)

	diffContainerStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	// Diff syntax highlighting styles
	diffAddedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22C55E")) // Green for additions

	diffRemovedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")) // Red for deletions

	diffContextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")) // Gray for context

	diffHeaderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6")) // Blue for headers

	diffOptionStyle = lipgloss.NewStyle()

	diffSelectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	diffHelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")) // Light gray color
)

// NewDiffConfirmation creates a new diff confirmation dialog
func NewDiffConfirmation(title, filePath, diffContent, executionID string, width, height int) DiffConfirmationModel {
	// Calculate max scroll based on diff content
	diffLines := strings.Split(diffContent, "\n")
	maxDiffHeight := height - 12 // Reserve space for title, options, help text, etc.
	if maxDiffHeight < 5 {
		maxDiffHeight = 5
	}
	
	maxScroll := len(diffLines) - maxDiffHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	return DiffConfirmationModel{
		title:         title,
		filePath:      filePath,
		diffContent:   diffContent,
		executionID:   executionID,
		selectedIndex: 0, // Default to "Yes"
		width:         width,
		height:        height,
		scrollOffset:  0,
		maxScroll:     maxScroll,
	}
}

// Init initializes the diff confirmation dialog (required by tea.Model interface)
func (m DiffConfirmationModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the diff confirmation dialog
func (m DiffConfirmationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				return diffConfirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   true,
				}
			}
		case "2", "esc":
			// Direct selection: No
			return m, func() tea.Msg {
				return diffConfirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   false,
				}
			}
		case "enter":
			// Confirm current selection
			confirmed := m.selectedIndex == 0 // Yes=0, No=1
			return m, func() tea.Msg {
				return diffConfirmationResponseMsg{
					executionID: m.executionID,
					confirmed:   confirmed,
				}
			}
		}
	}
	return m, nil
}

// View renders the diff confirmation dialog
func (m DiffConfirmationModel) View() string {
	// Prepare option rendering
	var yesOption, noOption string
	
	if m.selectedIndex == 0 {
		// Yes is selected
		yesOption = diffSelectedStyle.Render("▶ 1. Yes - Apply changes")
		noOption = diffOptionStyle.Render("  2. No  - Cancel ") + diffHelpStyle.Render("(or Esc)")
	} else {
		// No is selected
		yesOption = diffOptionStyle.Render("  1. Yes - Apply changes")
		noOption = diffSelectedStyle.Render("▶ 2. No  - Cancel ") + diffHelpStyle.Render("(or Esc)")
	}

	// Render diff content with syntax highlighting
	styledDiff := m.renderStyledDiff()

	// Build the complete dialog
	title := diffTitleStyle.Render(m.title)
	filePath := diffFilePathStyle.Render(m.filePath)
	
	changesLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render("Changes to be made:")

	// Create help text with scroll indicators
	helpText := diffHelpStyle.Render("Use ↑/↓ or 1/2 to select, Enter to confirm")
	if m.maxScroll > 0 {
		scrollInfo := fmt.Sprintf("(PgUp/PgDn to scroll diff, showing %d-%d of %d lines)", 
			m.scrollOffset+1, 
			min(m.scrollOffset+m.getDiffDisplayHeight(), len(strings.Split(m.diffContent, "\n"))),
			len(strings.Split(m.diffContent, "\n")))
		helpText += "\n" + diffHelpStyle.Render(scrollInfo)
	}

	content := fmt.Sprintf("%s\n%s\n\n%s\n%s\n\n%s\n%s\n\n%s", 
		title, filePath, changesLabel, styledDiff, yesOption, noOption, helpText)

	// Apply styling and return
	dialogWidth := m.width - 6 // Account for padding and borders
	if dialogWidth < 60 {
		dialogWidth = 60 // Minimum width for diff display
	}

	return diffConfirmationStyle.Width(dialogWidth).Render(content)
}

// renderStyledDiff applies syntax highlighting to the diff content
func (m DiffConfirmationModel) renderStyledDiff() string {
	if m.diffContent == "" {
		return diffContextStyle.Render("No changes to display")
	}

	lines := strings.Split(m.diffContent, "\n")
	
	// Calculate visible lines based on scroll offset
	displayHeight := m.getDiffDisplayHeight()
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
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			styledLine = diffHeaderStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			styledLine = diffHeaderStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			styledLine = diffAddedStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			styledLine = diffRemovedStyle.Render(line)
		default:
			styledLine = diffContextStyle.Render(line)
		}
		styledLines = append(styledLines, styledLine)
	}

	diffContent := strings.Join(styledLines, "\n")
	
	// Calculate container width for the diff
	containerWidth := m.width - 12 // Account for dialog padding, borders, and diff container borders
	if containerWidth < 40 {
		containerWidth = 40
	}

	return diffContainerStyle.Width(containerWidth).Render(diffContent)
}

// getDiffDisplayHeight calculates how many lines of diff to show based on dialog height
func (m DiffConfirmationModel) getDiffDisplayHeight() int {
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