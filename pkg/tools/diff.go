package tools

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/pmezard/go-difflib/difflib"
)

// DiffGenerator handles unified diff generation for file operations
type DiffGenerator struct {
	fileManager fileops.Manager
}

// NewDiffGenerator creates a new diff generator
func NewDiffGenerator(fileManager fileops.Manager) *DiffGenerator {
	return &DiffGenerator{
		fileManager: fileManager,
	}
}

// GenerateUnifiedDiff creates a unified diff between existing file content and new content
func (d *DiffGenerator) GenerateUnifiedDiff(filePath, newContent string) (string, error) {
	// Check if file exists
	fileExists := d.fileManager.FileExists(filePath)
	
	// Read existing content if file exists
	var oldContent string
	if fileExists {
		oldBytes, err := d.fileManager.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("error reading existing file: %w", err)
		}
		oldContent = string(oldBytes)
	}

	// Handle case where both old and new content are the same
	if oldContent == newContent {
		return "", fmt.Errorf("no changes detected - file content is identical")
	}

	// If file doesn't exist, generate new file diff
	if !fileExists {
		return d.generateNewFileDiff(filePath, newContent), nil
	}

	// Generate unified diff for existing file modifications
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: filePath,
		ToFile:   filePath,
		Context:  3, // Show 3 lines of context around changes
		Eol:      "\n",
	}

	diffText, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "", fmt.Errorf("error generating diff: %w", err)
	}

	return diffText, nil
}

// generateNewFileDiff creates a diff representation for new file creation
func (d *DiffGenerator) generateNewFileDiff(filePath, content string) string {
	var diff strings.Builder
	
	// Header for new file
	diff.WriteString(fmt.Sprintf("--- /dev/null\n"))
	diff.WriteString(fmt.Sprintf("+++ %s\n", filePath))
	diff.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(strings.Split(content, "\n"))))
	
	// Add all lines as additions
	for _, line := range strings.Split(content, "\n") {
		// Skip the last empty line that Split creates
		if line == "" && strings.HasSuffix(content, "\n") {
			continue
		}
		diff.WriteString(fmt.Sprintf("+%s\n", line))
	}
	
	return diff.String()
}

// DiffSummary provides a summary of changes in a diff
type DiffSummary struct {
	FilePath     string
	IsNewFile    bool
	IsModified   bool
	LinesAdded   int
	LinesRemoved int
	TotalLines   int
}

// AnalyzeDiff parses a unified diff and returns a summary of changes
func (d *DiffGenerator) AnalyzeDiff(diffContent string) DiffSummary {
	lines := strings.Split(diffContent, "\n")
	summary := DiffSummary{}
	
	for _, line := range lines {
		if strings.HasPrefix(line, "+++") {
			// Extract file path from +++ line
			parts := strings.Fields(line)
			if len(parts) > 1 {
				summary.FilePath = parts[1]
			}
		} else if strings.HasPrefix(line, "--- /dev/null") {
			summary.IsNewFile = true
		} else if strings.HasPrefix(line, "---") && !strings.Contains(line, "/dev/null") {
			summary.IsModified = true
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			summary.LinesAdded++
			summary.TotalLines++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			summary.LinesRemoved++
			summary.TotalLines++
		}
	}
	
	return summary
}

// FormatDiffForDisplay applies syntax highlighting and formatting for terminal display
func (d *DiffGenerator) FormatDiffForDisplay(diffContent string) string {
	if diffContent == "" {
		return "No changes to display"
	}
	
	lines := strings.Split(diffContent, "\n")
	var formatted strings.Builder
	
	for _, line := range lines {
		if line == "" {
			formatted.WriteString("\n")
			continue
		}
		
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// File headers - will be styled blue in the UI
			formatted.WriteString(line + "\n")
		case strings.HasPrefix(line, "@@"):
			// Hunk headers - will be styled blue in the UI  
			formatted.WriteString(line + "\n")
		case strings.HasPrefix(line, "+"):
			// Additions - will be styled green in the UI
			formatted.WriteString(line + "\n")
		case strings.HasPrefix(line, "-"):
			// Deletions - will be styled red in the UI
			formatted.WriteString(line + "\n")
		default:
			// Context lines - will be styled gray in the UI
			formatted.WriteString(line + "\n")
		}
	}
	
	return strings.TrimSuffix(formatted.String(), "\n")
}