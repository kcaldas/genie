package formatter_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kcaldas/genie/cmd/tui/formatter"
	"github.com/kcaldas/genie/cmd/tui/theme"
	"github.com/stretchr/testify/assert"
)

// createTestStyles creates a minimal styles struct for testing
func createTestStyles() theme.Styles {
	return theme.Styles{
		DiffAdded:        lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")),
		DiffRemoved:      lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")),
		DiffContext:      lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")),
		DiffHeader:       lipgloss.NewStyle().Foreground(lipgloss.Color("#0000FF")),
		ConfirmationHelp: lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")),
	}
}

func TestNewDiffFormatter(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	assert.NotNil(t, formatter)
}

func TestFormatDiff_EmptyContent(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	result := formatter.FormatDiff("", 0, 10)
	assert.Empty(t, result)
}

func TestFormatDiff_BasicFormatting(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	content := strings.Join([]string{
		"@@ -1,3 +1,4 @@",
		" context line 1",
		"-removed line",
		"+added line",
		" context line 2",
	}, "\n")
	
	result := formatter.FormatDiff(content, 0, 10)
	
	// Verify all lines are present (we can't easily test styling without complex setup)
	lines := strings.Split(result, "\n")
	assert.Len(t, lines, 5)
	assert.Contains(t, lines[0], "@@")
	assert.Contains(t, lines[1], "context line 1")
	assert.Contains(t, lines[2], "removed line")
	assert.Contains(t, lines[3], "added line")
	assert.Contains(t, lines[4], "context line 2")
}

func TestFormatDiff_Pagination(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	content := strings.Join([]string{
		"line 1",
		"line 2", 
		"line 3",
		"line 4",
		"line 5",
	}, "\n")
	
	// Test first page
	result := formatter.FormatDiff(content, 0, 3)
	lines := strings.Split(result, "\n")
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "line 1")
	assert.Contains(t, lines[2], "line 3")
	
	// Test second page
	result = formatter.FormatDiff(content, 2, 3)
	lines = strings.Split(result, "\n")
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "line 3")
	assert.Contains(t, lines[2], "line 5")
	
	// Test beyond content
	result = formatter.FormatDiff(content, 10, 3)
	assert.Empty(t, result)
}

func TestFormatDiff_PartialLastPage(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	content := strings.Join([]string{
		"line 1",
		"line 2",
		"line 3",
	}, "\n")
	
	// Request more lines than available
	result := formatter.FormatDiff(content, 1, 5)
	lines := strings.Split(result, "\n")
	assert.Len(t, lines, 2) // Only 2 lines available from startLine=1
	assert.Contains(t, lines[0], "line 2")
	assert.Contains(t, lines[1], "line 3")
}

func TestHighlightDiffLine_DifferentPrefixes(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	testCases := []struct {
		name     string
		input    string
		prefix   string
		expected string
	}{
		{
			name:     "added line",
			input:    "+new content",
			prefix:   "+",
			expected: "+    new content", // Tab converted to spaces by lipgloss
		},
		{
			name:     "removed line", 
			input:    "-old content",
			prefix:   "-",
			expected: "-    old content", // Tab converted to spaces by lipgloss
		},
		{
			name:     "header line",
			input:    "@@ -1,3 +1,4 @@",
			prefix:   "@",
			expected: "@@ -1,3 +1,4 @@",
		},
		{
			name:     "context line",
			input:    " unchanged content",
			prefix:   " ",
			expected: " unchanged content",
		},
		{
			name:     "empty line",
			input:    "",
			prefix:   "",
			expected: "",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test individual line formatting by creating single-line content
			result := formatter.FormatDiff(tc.input, 0, 1)
			
			if tc.input == "" {
				assert.Empty(t, result)
			} else {
				// Verify the content is preserved (styling is applied but we can't easily test it)
				assert.Contains(t, result, tc.expected)
			}
		})
	}
}

func TestFormatScrollInfo_NoScrollNeeded(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	// When all lines fit, no scroll info should be shown
	result := formatter.FormatScrollInfo(0, 5, 5)
	assert.Empty(t, result)
	
	result = formatter.FormatScrollInfo(0, 10, 5)
	assert.Empty(t, result)
}

func TestFormatScrollInfo_ScrollNeeded(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	result := formatter.FormatScrollInfo(0, 5, 10)
	assert.Contains(t, result, "(Line 1-5 of 10)")
	
	result = formatter.FormatScrollInfo(5, 10, 15)
	assert.Contains(t, result, "(Line 6-10 of 15)")
	
	result = formatter.FormatScrollInfo(2, 7, 12)
	assert.Contains(t, result, "(Line 3-7 of 12)")
}

func TestFormatDiff_RealWorldExample(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	// Simulate a real diff
	realDiff := strings.Join([]string{
		"@@ -12,7 +12,8 @@ func main() {",
		" \tif err != nil {",
		" \t\tlog.Fatal(err)",
		" \t}",
		"-\tfmt.Println(\"Hello World\")",
		"+\tfmt.Println(\"Hello Genie\")",
		"+\tfmt.Println(\"Welcome!\")",
		" \treturn",
		" }",
	}, "\n")
	
	// Test formatting first 5 lines
	result := formatter.FormatDiff(realDiff, 0, 5)
	lines := strings.Split(result, "\n")
	assert.Len(t, lines, 5)
	
	// Verify header line
	assert.Contains(t, lines[0], "@@")
	
	// Verify context lines
	assert.Contains(t, lines[1], "if err != nil")
	
	// Verify removed line
	assert.Contains(t, lines[4], "Hello World")
	
	// Test pagination - next page
	result = formatter.FormatDiff(realDiff, 5, 3)
	lines = strings.Split(result, "\n")
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "Hello Genie")
	assert.Contains(t, lines[1], "Welcome!")
}

func TestFormatDiff_SingleLineContent(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	content := "+single added line"
	
	result := formatter.FormatDiff(content, 0, 1)
	assert.Contains(t, result, "+    single added line")
	
	// Test beyond single line
	result = formatter.FormatDiff(content, 1, 1)
	assert.Empty(t, result)
}

func TestFormatDiff_PreservesExactContent(t *testing.T) {
	styles := createTestStyles()
	formatter := formatter.NewDiffFormatter(styles)
	
	// Test with special characters and content preservation
	content := strings.Join([]string{
		"+indented with tabs",
		"-indented with spaces",
		" mixed whitespace",
		"@special chars: !@#$%^&*()",
	}, "\n")
	
	result := formatter.FormatDiff(content, 0, 10)
	
	// Verify content preservation and indentation formatting
	assert.Contains(t, result, "+    indented with tabs")
	assert.Contains(t, result, "-    indented with spaces") 
	assert.Contains(t, result, "mixed whitespace")
	assert.Contains(t, result, "special chars: !@#$%^&*()")
}