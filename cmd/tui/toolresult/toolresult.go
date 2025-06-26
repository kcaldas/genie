// Package toolresult provides a tool execution result component for Genie TUI.
// It follows the Bubble Tea component patterns for consistent behavior.
package toolresult

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents a tool execution result component following Bubbles patterns
type Model struct {
	toolName   string
	parameters map[string]any
	success    bool
	result     map[string]any
	expanded   bool
}

// Styles for tool result rendering
var (
	indicatorStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#10B981")) // Success green

	toolNameStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#6366F1")) // Indigo

	summaryStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")) // Gray

	keyStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#9CA3AF")) // Light gray

	valueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")) // Red

	truncatedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")) // Light gray
)

// New creates a new tool result component following Bubbles patterns
func New(toolName string, parameters map[string]any, success bool, result map[string]any, expanded bool) Model {
	return Model{
		toolName:   toolName,
		parameters: parameters,
		success:    success,
		result:     result,
		expanded:   expanded,
	}
}

// Init initializes the tool result (required by tea.Model interface)
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the tool result (currently read-only)
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// Tool results are currently read-only components
	// Future: could add expansion/collapse functionality here
	return m, nil
}

// SetExpanded sets the expansion state of the tool result
func (m Model) SetExpanded(expanded bool) Model {
	m.expanded = expanded
	return m
}

// ToggleExpanded toggles the expansion state of the tool result
func (m Model) ToggleExpanded() Model {
	m.expanded = !m.expanded
	return m
}

// View renders the tool result component
func (m Model) View() string {
	var result strings.Builder

	// Indicator and tool name
	indicator := "●"
	if m.success {
		result.WriteString(indicatorStyle.Render(indicator))
	} else {
		result.WriteString(indicatorStyle.Foreground(lipgloss.Color("#EF4444")).Render(indicator))
	}

	result.WriteString(" ")
	result.WriteString(toolNameStyle.Render(m.toolName))

	// Add file/command info for better visual identification
	targetInfo := m.getTargetInfo()
	if targetInfo != "" {
		result.WriteString(" ")
		result.WriteString(summaryStyle.Render(targetInfo))
	}

	// Add summary based on result type
	summary := m.getSummary()
	if summary != "" {
		result.WriteString(" ")
		result.WriteString(summaryStyle.Render(summary))
	}

	// Show expanded content if toggled, or compact content if appropriate
	if m.result != nil {
		if m.expanded {
			result.WriteString("\n")
			result.WriteString(m.renderDetailedContent())
		} else if m.shouldShowContent() {
			result.WriteString("\n")
			result.WriteString(m.renderCompactContent())
		}
	}

	return result.String()
}

// getSummary returns a brief summary of the tool result
func (m Model) getSummary() string {
	if m.result == nil {
		return ""
	}

	// Handle different tool types with specific summaries
	switch m.toolName {
	case "readFile":
		if content, ok := m.result["content"].(string); ok {
			if content == "" {
				return "(empty file)"
			}
			lines := strings.Split(content, "\n")
			return fmt.Sprintf("(%d lines)", len(lines))
		}
		if errorMsg, ok := m.result["error"].(string); ok {
			return fmt.Sprintf("(error: %s)", errorMsg)
		}
	case "listFiles":
		if files, ok := m.result["files"].(string); ok {
			if files == "" {
				return "(empty directory)"
			}
			fileList := strings.Split(strings.TrimSpace(files), "\n")
			return fmt.Sprintf("(%d items)", len(fileList))
		}
	case "runBashCommand":
		if output, ok := m.result["output"].(string); ok {
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

// renderDetailedContent renders the full result content with nice formatting
func (m Model) renderDetailedContent() string {
	var content strings.Builder

	maxContentLength := 300 // Limit content display

	for key, value := range m.result {
		if key == "success" {
			continue // Skip success as it's shown by indicator
		}

		valueStr := fmt.Sprintf("%v", value)
		if valueStr == "" {
			continue
		}

		content.WriteString("    ") // 4-space indent
		content.WriteString(keyStyle.Render(key + ":"))
		content.WriteString(" ")

		// Handle different content types
		if key == "error" {
			content.WriteString(errorStyle.Render(valueStr))
		} else if key == "content" || key == "output" {
			// For file content or command output, show preview
			if len(valueStr) > maxContentLength {
				preview := valueStr[:maxContentLength]
				content.WriteString(valueStyle.Render(preview))
				content.WriteString(truncatedStyle.Render("... (truncated)"))
			} else {
				content.WriteString(valueStyle.Render(valueStr))
			}
		} else if key == "files" {
			// For file listings, show in a nice format
			if strings.Contains(valueStr, "\n") {
				files := strings.Split(strings.TrimSpace(valueStr), "\n")
				content.WriteString("\n")
				for i, file := range files {
					if i > 10 { // Limit to first 10 files
						content.WriteString("      ")
						content.WriteString(truncatedStyle.Render(fmt.Sprintf("... and %d more", len(files)-i)))
						break
					}
					content.WriteString("      ") // Extra indent for file list
					content.WriteString(valueStyle.Render(file))
					if i < len(files)-1 {
						content.WriteString("\n")
					}
				}
			} else {
				content.WriteString(valueStyle.Render(valueStr))
			}
		} else {
			// For any other content, apply truncation
			if len(valueStr) > maxContentLength {
				preview := valueStr[:maxContentLength]
				content.WriteString(valueStyle.Render(preview))
				content.WriteString(truncatedStyle.Render("... (truncated)"))
			} else {
				content.WriteString(valueStyle.Render(valueStr))
			}
		}

		content.WriteString("\n")
	}

	return content.String()
}

// getTargetInfo extracts parameters in a simplified format for display
func (m Model) getTargetInfo() string {
	if m.parameters == nil {
		return ""
	}

	var params []string

	// Add key parameters in simplified format
	for key, value := range m.parameters {
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
func (m Model) shouldShowContent() bool {
	if m.result == nil {
		return false
	}

	// Show content for errors always
	if _, hasError := m.result["error"]; hasError {
		return true
	}

	// For short content, show it
	if content, ok := m.result["content"].(string); ok {
		return len(content) <= 100 // Show short file contents
	}

	// For short file lists, show them
	if files, ok := m.result["files"].(string); ok {
		fileList := strings.Split(strings.TrimSpace(files), "\n")
		return len(fileList) <= 5 // Show short directory listings
	}

	// For short command output, show it
	if output, ok := m.result["output"].(string); ok {
		return len(output) <= 100 // Show short command outputs
	}

	return false
}

// renderCompactContent renders content in a compact, readable way
func (m Model) renderCompactContent() string {
	var content strings.Builder

	for key, value := range m.result {
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
			content.WriteString(errorStyle.Render(valueStr))
		} else if key == "files" {
			// Show files in a compact list
			files := strings.Split(strings.TrimSpace(valueStr), "\n")
			for i, file := range files {
				if i > 0 {
					content.WriteString("\n    ")
				}
				content.WriteString(valueStyle.Render(file))
			}
		} else {
			// For content/output, show directly without key label
			content.WriteString(valueStyle.Render(valueStr))
		}

		content.WriteString("\n")
	}

	return content.String()
}

// String returns a string representation (for when used as a simple component)
func (m Model) String() string {
	return m.View()
}

// GetToolName returns the tool name
func (m Model) GetToolName() string {
	return m.toolName
}

// GetSuccess returns whether the tool execution was successful
func (m Model) GetSuccess() bool {
	return m.success
}

// IsExpanded returns whether the tool result is expanded
func (m Model) IsExpanded() bool {
	return m.expanded
}