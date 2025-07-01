package presentation

import (
	"strings"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

// DiffFormatter formats diff output with theme-aware colors
type DiffFormatter struct {
	theme *types.Theme
}

// NewDiffFormatter creates a new diff formatter with the given theme
func NewDiffFormatter(theme *types.Theme) *DiffFormatter {
	return &DiffFormatter{theme: theme}
}

// Format applies theme colors to diff content
func (f *DiffFormatter) Format(content string) string {
	if f.theme == nil {
		return content
	}

	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		formattedLine := f.formatLine(line)
		result = append(result, formattedLine)
	}

	return strings.Join(result, "\n")
}

// formatLine applies appropriate color to a diff line based on its prefix
func (f *DiffFormatter) formatLine(line string) string {
	// Get ANSI colors from theme
	addColor := ConvertColorToAnsi(f.theme.Success)     // Green for additions
	removeColor := ConvertColorToAnsi(f.theme.Error)    // Red for removals
	headerColor := ConvertColorToAnsi(f.theme.Primary)  // Primary color for headers
	lineNumColor := ConvertColorToAnsi(f.theme.Muted)   // Muted for line numbers
	reset := "\033[0m"

	// Handle different diff line types
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		// File headers
		return headerColor + line + reset
	
	case strings.HasPrefix(line, "@@"):
		// Hunk headers (line numbers)
		return lineNumColor + line + reset
	
	case strings.HasPrefix(line, "+"):
		// Added lines
		return addColor + line + reset
	
	case strings.HasPrefix(line, "-"):
		// Removed lines
		return removeColor + line + reset
	
	case strings.HasPrefix(line, "diff --git"):
		// Git diff header
		return headerColor + line + reset
	
	case strings.HasPrefix(line, "index "):
		// Git index line
		return lineNumColor + line + reset
	
	case strings.HasPrefix(line, "new file mode"):
		// New file indicator
		return addColor + line + reset
	
	case strings.HasPrefix(line, "deleted file mode"):
		// Deleted file indicator
		return removeColor + line + reset
	
	case strings.HasPrefix(line, "Binary files"):
		// Binary file indicator
		return lineNumColor + line + reset
	
	default:
		// Context lines or other content
		return line
	}
}

// FormatUnified formats a unified diff with colors
func (f *DiffFormatter) FormatUnified(oldContent, newContent string, oldName, newName string) string {
	// This is a placeholder for generating unified diffs
	// In practice, you might want to use a proper diff library
	var result strings.Builder

	// Header
	headerColor := ConvertColorToAnsi(f.theme.Primary)
	reset := "\033[0m"
	
	result.WriteString(headerColor + "--- " + oldName + reset + "\n")
	result.WriteString(headerColor + "+++ " + newName + reset + "\n")

	// For now, just show the content change
	// In a real implementation, you'd compute the actual diff
	if oldContent != "" {
		removeColor := ConvertColorToAnsi(f.theme.Error)
		for _, line := range strings.Split(oldContent, "\n") {
			result.WriteString(removeColor + "- " + line + reset + "\n")
		}
	}

	if newContent != "" {
		addColor := ConvertColorToAnsi(f.theme.Success)
		for _, line := range strings.Split(newContent, "\n") {
			result.WriteString(addColor + "+ " + line + reset + "\n")
		}
	}

	return result.String()
}

// FormatSideBySide formats a side-by-side diff (simplified version)
func (f *DiffFormatter) FormatSideBySide(oldContent, newContent string, width int) string {
	// This is a simplified side-by-side formatter
	// A full implementation would properly align and handle line differences
	
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}
	
	halfWidth := width / 2 - 2
	removeColor := ConvertColorToAnsi(f.theme.Error)
	addColor := ConvertColorToAnsi(f.theme.Success)
	reset := "\033[0m"
	
	var result strings.Builder
	
	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		
		if i < len(oldLines) {
			oldLine = oldLines[i]
			if len(oldLine) > halfWidth {
				oldLine = oldLine[:halfWidth-3] + "..."
			}
		}
		
		if i < len(newLines) {
			newLine = newLines[i]
			if len(newLine) > halfWidth {
				newLine = newLine[:halfWidth-3] + "..."
			}
		}
		
		// Format the line
		if oldLine != "" && newLine != "" && oldLine != newLine {
			// Changed line
			result.WriteString(removeColor + oldLine + reset)
			result.WriteString(strings.Repeat(" ", halfWidth-len(oldLine)+2))
			result.WriteString(addColor + newLine + reset)
		} else if oldLine != "" && newLine == "" {
			// Removed line
			result.WriteString(removeColor + oldLine + reset)
		} else if oldLine == "" && newLine != "" {
			// Added line
			result.WriteString(strings.Repeat(" ", halfWidth+2))
			result.WriteString(addColor + newLine + reset)
		} else {
			// Unchanged line
			result.WriteString(oldLine)
			if newLine != "" {
				result.WriteString(strings.Repeat(" ", halfWidth-len(oldLine)+2))
				result.WriteString(newLine)
			}
		}
		result.WriteString("\n")
	}
	
	return result.String()
}