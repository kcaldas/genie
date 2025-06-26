// Package formatter provides formatting utilities for displaying content in Genie TUI.
package formatter

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/theme"
)

// DiffFormatter handles formatting of diff content for display
type DiffFormatter struct {
	styles theme.Styles
}

// NewDiffFormatter creates a new diff formatter with the provided styles
func NewDiffFormatter(styles theme.Styles) *DiffFormatter {
	return &DiffFormatter{
		styles: styles,
	}
}

// FormatDiff formats diff content for display with proper syntax highlighting
func (f *DiffFormatter) FormatDiff(content string, startLine, maxLines int) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	
	// Calculate which lines to show
	endLine := startLine + maxLines
	if endLine > len(lines) {
		endLine = len(lines)
	}
	
	if startLine >= len(lines) {
		return ""
	}
	
	visibleLines := lines[startLine:endLine]
	
	// Apply syntax highlighting to each line
	for i, line := range visibleLines {
		visibleLines[i] = f.highlightDiffLine(line)
	}
	
	return strings.Join(visibleLines, "\n")
}

// highlightDiffLine applies syntax highlighting to a single diff line based on its prefix
func (f *DiffFormatter) highlightDiffLine(line string) string {
	if len(line) == 0 {
		return line
	}
	
	switch line[0] {
	case '+':
		// Add tab between + and content for better readability
		content := line[1:]
		return f.styles.DiffAdded.Render("+" + "\t" + content)
	case '-':
		// Add tab between - and content for better readability
		content := line[1:]
		return f.styles.DiffRemoved.Render("-" + "\t" + content)
	case '@':
		return f.styles.DiffHeader.Render(line)
	default:
		return f.styles.DiffContext.Render(line)
	}
}

// FormatScrollInfo formats scroll information display
func (f *DiffFormatter) FormatScrollInfo(startLine, endLine, totalLines int) string {
	if totalLines <= endLine-startLine {
		return ""
	}
	
	return f.styles.ConfirmationHelp.Render(
		fmt.Sprintf("(Line %d-%d of %d)", startLine+1, endLine, totalLines),
	)
}