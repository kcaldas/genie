package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ToolResult represents a tool execution result component
type ToolResult struct {
	toolName   string
	parameters map[string]any
	success    bool
	result     map[string]any
	width      int
	expanded   bool
}

// NewToolResult creates a new tool result component
func NewToolResult(toolName string, parameters map[string]any, success bool, result map[string]any, width int, expanded bool) ToolResult {
	return ToolResult{
		toolName:   toolName,
		parameters: parameters,
		success:    success,
		result:     result,
		width:      width,
		expanded:   expanded,
	}
}

// Init implements tea.Model
func (tr ToolResult) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model (not used for static display)
func (tr ToolResult) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return tr, nil
}

// View implements tea.Model
func (tr ToolResult) View() string {
	var styles = struct {
		indicator  lipgloss.Style
		toolName   lipgloss.Style
		summary    lipgloss.Style
		key        lipgloss.Style
		value      lipgloss.Style
		error      lipgloss.Style
		truncated  lipgloss.Style
		expandHint lipgloss.Style
	}{
		indicator: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")), // Success green
		toolName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#6366F1")), // Indigo
		summary: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")), // Gray
		key: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9CA3AF")), // Light gray
		value: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")),
		error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")), // Red
		truncated: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")), // Light gray
		expandHint: lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#9CA3AF")), // Light gray
	}

	if !tr.success {
		styles.indicator = styles.indicator.Foreground(lipgloss.Color("#EF4444")) // Error red
	}

	var result strings.Builder

	// Indicator and tool name
	indicator := "●"
	if tr.success {
		result.WriteString(styles.indicator.Render(indicator))
	} else {
		result.WriteString(styles.indicator.Render(indicator))
	}

	result.WriteString(" ")
	result.WriteString(styles.toolName.Render(tr.toolName))

	// Add file/command info for better visual identification
	targetInfo := tr.getTargetInfo()
	if targetInfo != "" {
		result.WriteString(" ")
		result.WriteString(styles.summary.Render(targetInfo))
	}

	// Add summary based on result type
	summary := tr.getSummary()
	if summary != "" {
		result.WriteString(" ")
		result.WriteString(styles.summary.Render(summary))
	}

	// Show expanded content if toggled, or compact content if appropriate
	if tr.result != nil {
		if tr.expanded {
			result.WriteString("\n")
			result.WriteString(tr.renderDetailedContent(styles))
		} else if tr.shouldShowContent() {
			result.WriteString("\n")
			result.WriteString(tr.renderCompactContent(styles))
		}
	}

	return result.String()
}

// getSummary returns a brief summary of the tool result
func (tr ToolResult) getSummary() string {
	if tr.result == nil {
		return ""
	}

	// Handle different tool types with specific summaries
	switch tr.toolName {
	case "readFile":
		if content, ok := tr.result["content"].(string); ok {
			if content == "" {
				return "(empty file)"
			}
			lines := strings.Split(content, "\n")
			return fmt.Sprintf("(%d lines)", len(lines))
		}
		if errorMsg, ok := tr.result["error"].(string); ok {
			return fmt.Sprintf("(error: %s)", errorMsg)
		}
	case "listFiles":
		if files, ok := tr.result["files"].(string); ok {
			if files == "" {
				return "(empty directory)"
			}
			fileList := strings.Split(strings.TrimSpace(files), "\n")
			return fmt.Sprintf("(%d items)", len(fileList))
		}
	case "runBashCommand":
		if output, ok := tr.result["output"].(string); ok {
			if output == "" {
				return "(no output)"
			}
			lines := strings.Split(strings.TrimSpace(output), "\n")
			return fmt.Sprintf("(%d lines output)", len(lines))
		}
	case "writeFile":
		return "(file written)"
	}

	return ""
}

// hasDetailedContent checks if there's content worth expanding
func (tr ToolResult) hasDetailedContent() bool {
	if tr.result == nil {
		return false
	}

	// Check if there's meaningful content beyond just success
	for key, value := range tr.result {
		if key != "success" {
			if str, ok := value.(string); ok && str != "" {
				return true
			}
			if value != nil {
				return true
			}
		}
	}
	return false
}

// renderDetailedContent renders the full result content with nice formatting
func (tr ToolResult) renderDetailedContent(styles struct {
	indicator  lipgloss.Style
	toolName   lipgloss.Style
	summary    lipgloss.Style
	key        lipgloss.Style
	value      lipgloss.Style
	error      lipgloss.Style
	truncated  lipgloss.Style
	expandHint lipgloss.Style
}) string {
	var content strings.Builder

	maxContentLength := 300 // Limit content display

	for key, value := range tr.result {
		if key == "success" {
			continue // Skip success as it's shown by indicator
		}

		valueStr := fmt.Sprintf("%v", value)
		if valueStr == "" {
			continue
		}

		content.WriteString("    ") // 4-space indent
		content.WriteString(styles.key.Render(key + ":"))
		content.WriteString(" ")

		// Handle different content types
		if key == "error" {
			content.WriteString(styles.error.Render(valueStr))
		} else if key == "content" || key == "output" {
			// For file content or command output, show preview
			if len(valueStr) > maxContentLength {
				preview := valueStr[:maxContentLength]
				content.WriteString(styles.value.Render(preview))
				content.WriteString(styles.truncated.Render("... (truncated)"))
			} else {
				content.WriteString(styles.value.Render(valueStr))
			}
		} else if key == "files" {
			// For file listings, show in a nice format
			if strings.Contains(valueStr, "\n") {
				files := strings.Split(strings.TrimSpace(valueStr), "\n")
				content.WriteString("\n")
				for i, file := range files {
					if i > 10 { // Limit to first 10 files
						content.WriteString("      ")
						content.WriteString(styles.truncated.Render(fmt.Sprintf("... and %d more", len(files)-i)))
						break
					}
					content.WriteString("      ") // Extra indent for file list
					content.WriteString(styles.value.Render(file))
					if i < len(files)-1 {
						content.WriteString("\n")
					}
				}
			} else {
				content.WriteString(styles.value.Render(valueStr))
			}
		} else {
			content.WriteString(styles.value.Render(valueStr))
		}

		content.WriteString("\n")
	}

	return content.String()
}

// getTargetInfo extracts parameters in a simplified format for display
func (tr ToolResult) getTargetInfo() string {
	if tr.parameters == nil {
		return ""
	}

	var params []string

	// Add key parameters in simplified format
	for key, value := range tr.parameters {
		if value == nil {
			continue
		}

		valueStr := fmt.Sprintf("%v", value)
		if valueStr == "" {
			continue
		}

		// Truncate long values
		if len(valueStr) > 40 {
			valueStr = valueStr[:40] + "..."
		}

		params = append(params, fmt.Sprintf("%s: %s", key, valueStr))
	}

	if len(params) == 0 {
		return ""
	}

	return fmt.Sprintf("(%s)", strings.Join(params, ", "))
}

// shouldShowContent determines if we should show detailed content in compact mode
func (tr ToolResult) shouldShowContent() bool {
	if tr.result == nil {
		return false
	}

	// Show content for errors always
	if _, hasError := tr.result["error"]; hasError {
		return true
	}

	// For short content, show it
	if content, ok := tr.result["content"].(string); ok {
		return len(content) <= 100 // Show short file contents
	}

	// For short file lists, show them
	if files, ok := tr.result["files"].(string); ok {
		fileList := strings.Split(strings.TrimSpace(files), "\n")
		return len(fileList) <= 5 // Show short directory listings
	}

	// For short command output, show it
	if output, ok := tr.result["output"].(string); ok {
		return len(output) <= 100 // Show short command outputs
	}

	return false
}

// renderCompactContent renders content in a compact, readable way
func (tr ToolResult) renderCompactContent(styles struct {
	indicator  lipgloss.Style
	toolName   lipgloss.Style
	summary    lipgloss.Style
	key        lipgloss.Style
	value      lipgloss.Style
	error      lipgloss.Style
	truncated  lipgloss.Style
	expandHint lipgloss.Style
}) string {
	var content strings.Builder

	for key, value := range tr.result {
		if key == "success" {
			continue // Skip success as it's shown by indicator
		}

		valueStr := fmt.Sprintf("%v", value)
		if valueStr == "" {
			continue
		}

		content.WriteString("    ") // 4-space indent

		// Handle different content types
		if key == "error" {
			content.WriteString(styles.error.Render(valueStr))
		} else if key == "files" {
			// Show files in a compact list
			files := strings.Split(strings.TrimSpace(valueStr), "\n")
			for i, file := range files {
				if i > 0 {
					content.WriteString("\n    ")
				}
				content.WriteString(styles.value.Render(file))
			}
		} else {
			// For content/output, show directly without key label
			content.WriteString(styles.value.Render(valueStr))
		}

		content.WriteString("\n")
	}

	return content.String()
}

// String returns a string representation (for when used as a simple component)
func (tr ToolResult) String() string {
	return tr.View()
}
