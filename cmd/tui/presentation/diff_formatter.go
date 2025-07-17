package presentation

import (
	"strings"
)

// DiffFormatter formats diff output with diff-specific colors
type DiffFormatter struct {
	diffTheme *DiffTheme
}

// NewDiffFormatter creates a new diff formatter with the given diff theme
func NewDiffFormatter(diffTheme *DiffTheme) *DiffFormatter {
	return &DiffFormatter{diffTheme: diffTheme}
}

// Format applies diff theme colors to diff content
func (f *DiffFormatter) Format(content string) string {
	if f.diffTheme == nil {
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
	// Get ANSI colors from diff theme
	addFg := ConvertColorToAnsi(f.diffTheme.AddedFg)
	addBg := ConvertColorToAnsiBg(f.diffTheme.AddedBg)
	removeFg := ConvertColorToAnsi(f.diffTheme.RemovedFg)
	removeBg := ConvertColorToAnsiBg(f.diffTheme.RemovedBg)
	headerFg := ConvertColorToAnsi(f.diffTheme.HeaderFg)
	headerBg := ConvertColorToAnsiBg(f.diffTheme.HeaderBg)
	hunkFg := ConvertColorToAnsi(f.diffTheme.HunkFg)
	hunkBg := ConvertColorToAnsiBg(f.diffTheme.HunkBg)
	contextFg := ConvertColorToAnsi(f.diffTheme.ContextFg)
	contextBg := ConvertColorToAnsiBg(f.diffTheme.ContextBg)
	reset := "\033[0m"

	// Handle different diff line types
	// Check specific patterns first, then general ones
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		// File headers (3 chars, more specific than single +/-)
		return headerBg + headerFg + line + reset
	
	case strings.HasPrefix(line, "@@"):
		// Hunk headers (line numbers)
		return hunkBg + hunkFg + line + reset
	
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		// Added lines (but not +++ headers)
		return addBg + addFg + line + reset
	
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		// Removed lines (but not --- headers)
		return removeBg + removeFg + line + reset
	
	case strings.HasPrefix(line, "diff --git"):
		// Git diff header
		return headerBg + headerFg + line + reset
	
	case strings.HasPrefix(line, "index "):
		// Git index line
		return hunkBg + hunkFg + line + reset
	
	case strings.HasPrefix(line, "new file mode"):
		// New file indicator
		return addBg + addFg + line + reset
	
	case strings.HasPrefix(line, "deleted file mode"):
		// Deleted file indicator
		return removeBg + removeFg + line + reset
	
	case strings.HasPrefix(line, "Binary files"):
		// Binary file indicator
		return hunkBg + hunkFg + line + reset
	
	default:
		// Context lines or other content
		if contextFg != "" || contextBg != "" {
			return contextBg + contextFg + line + reset
		}
		return line
	}
}

// FormatUnified formats a unified diff with colors
func (f *DiffFormatter) FormatUnified(oldContent, newContent string, oldName, newName string) string {
	// This is a placeholder for generating unified diffs
	// In practice, you might want to use a proper diff library
	var result strings.Builder

	// Header
	headerFg := ConvertColorToAnsi(f.diffTheme.HeaderFg)
	headerBg := ConvertColorToAnsiBg(f.diffTheme.HeaderBg)
	reset := "\033[0m"
	
	result.WriteString(headerBg + headerFg + "--- " + oldName + reset + "\n")
	result.WriteString(headerBg + headerFg + "+++ " + newName + reset + "\n")

	// For now, just show the content change
	// In a real implementation, you'd compute the actual diff
	if oldContent != "" {
		removeFg := ConvertColorToAnsi(f.diffTheme.RemovedFg)
		removeBg := ConvertColorToAnsiBg(f.diffTheme.RemovedBg)
		for _, line := range strings.Split(oldContent, "\n") {
			result.WriteString(removeBg + removeFg + "- " + line + reset + "\n")
		}
	}

	if newContent != "" {
		addFg := ConvertColorToAnsi(f.diffTheme.AddedFg)
		addBg := ConvertColorToAnsiBg(f.diffTheme.AddedBg)
		for _, line := range strings.Split(newContent, "\n") {
			result.WriteString(addBg + addFg + "+ " + line + reset + "\n")
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
	removeFg := ConvertColorToAnsi(f.diffTheme.RemovedFg)
	removeBg := ConvertColorToAnsiBg(f.diffTheme.RemovedBg)
	addFg := ConvertColorToAnsi(f.diffTheme.AddedFg)
	addBg := ConvertColorToAnsiBg(f.diffTheme.AddedBg)
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
			result.WriteString(removeBg + removeFg + oldLine + reset)
			result.WriteString(strings.Repeat(" ", halfWidth-len(oldLine)+2))
			result.WriteString(addBg + addFg + newLine + reset)
		} else if oldLine != "" && newLine == "" {
			// Removed line
			result.WriteString(removeBg + removeFg + oldLine + reset)
		} else if oldLine == "" && newLine != "" {
			// Added line
			result.WriteString(strings.Repeat(" ", halfWidth+2))
			result.WriteString(addBg + addFg + newLine + reset)
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