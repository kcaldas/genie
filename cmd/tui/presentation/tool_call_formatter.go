package presentation

import (
	"fmt"
	"sort"
	"strings"
	
	"github.com/kcaldas/genie/cmd/tui/types"
)

// FormatToolCall formats tool calls for display in the chat interface
func FormatToolCall(toolName string, params map[string]any, config *types.Config) string {
	// Special case for TodoWrite - just show "Updated Todos"
	if toolName == "TodoWrite" {
		return "Updated Todos"
	}

	// Get theme colors for formatting
	theme := GetThemeForMode(config.Theme, config.OutputMode)
	tertiaryColor := ConvertColorToAnsi(theme.TextTertiary)
	resetColor := "\033[0m"

	if len(params) == 0 {
		paramsText := "()"
		if tertiaryColor != "" {
			paramsText = tertiaryColor + paramsText + resetColor
		}
		return fmt.Sprintf("%s%s", toolName, paramsText)
	}

	var paramPairs []string
	for key, value := range params {
		// Format the value appropriately
		var valueStr string
		switch v := value.(type) {
		case string:
			// Truncate long strings
			if len(v) > 50 {
				valueStr = fmt.Sprintf(`"%s..."`, v[:50])
			} else {
				valueStr = fmt.Sprintf(`"%s"`, v)
			}
		case bool:
			valueStr = fmt.Sprintf("%t", v)
		case nil:
			valueStr = "null"
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		paramPairs = append(paramPairs, fmt.Sprintf("%s: %s", key, valueStr))
	}

	// Sort for consistent display
	sort.Strings(paramPairs)

	paramsText := fmt.Sprintf("(%s)", strings.Join(paramPairs, ", "))
	if tertiaryColor != "" {
		paramsText = tertiaryColor + paramsText + resetColor
	}

	return fmt.Sprintf("%s%s", toolName, paramsText)
}

// FormatToolResult formats the result of a tool execution for display in the chat interface
func FormatToolResult(toolName string, result map[string]any, todoFormatter *TodoFormatter, config *types.Config) string {
	if result == nil || len(result) == 0 {
		return ""
	}

	// Get theme colors
	theme := GetThemeForMode(config.Theme, config.OutputMode)
	tertiaryColor := ConvertColorToAnsi(theme.TextTertiary)
	resetColor := "\033[0m"

	// Handle writeFile tools with special diff formatting
	if toolName == "writeFile" {
		return formatWriteFileResult(result, config)
	}

	// Handle todo tools with special formatting
	if toolName == "TodoWrite" && todoFormatter != nil {
		// Use TodoFormatter for todo tools
		formattedTodos := todoFormatter.FormatTodoToolResult(result)

		// Add L-shaped formatting like other tools
		lines := strings.Split(strings.TrimSpace(formattedTodos), "\n")
		var formatted []string
		for i, line := range lines {
			if line != "" {
				if i == 0 {
					// First line gets the L-shaped character
					formatted = append(formatted, fmt.Sprintf("%s└─%s %s", tertiaryColor, resetColor, line))
				} else {
					// Subsequent lines just get indentation
					formatted = append(formatted, fmt.Sprintf("   %s", line))
				}
			}
		}
		return "\n" + strings.Join(formatted, "\n")
	}

	// Format the result preview with L-shaped symbol and tertiary color for other tools
	// Extract a preview from the result
	var preview string
	// Try to get a meaningful preview from common result fields
	if content, ok := result["content"].(string); ok && content != "" {
		preview = content
	} else if output, ok := result["output"].(string); ok && output != "" {
		preview = output
	} else if data, ok := result["data"].(string); ok && data != "" {
		preview = data
	} else {
		// Fallback to first string value found
		for _, v := range result {
			if str, ok := v.(string); ok && str != "" {
				preview = str
				break
			}
		}
	}

	if preview != "" {
		// Clean up the preview
		preview = strings.TrimSpace(preview)
		
		// Show first N lines of the preview (with smart truncation)
		const maxLines = 3
		lines := strings.Split(preview, "\n")
		
		// Remove empty lines from the end
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) == "" {
				lines = lines[:i]
			} else {
				break
			}
		}
		
		var resultLines []string
		for i, line := range lines {
			if i >= maxLines {
				break
			}
			// Trim long lines
			if len(line) > 80 {
				line = line[:77] + "..."
			}
			if i == 0 {
				// First line gets the L-shaped character
				resultLines = append(resultLines, fmt.Sprintf("%s└─ %s%s", tertiaryColor, line, resetColor))
			} else {
				// Subsequent lines get indentation
				resultLines = append(resultLines, fmt.Sprintf("%s   %s%s", tertiaryColor, line, resetColor))
			}
		}
		
		// Add truncation indicator if there are more lines
		if len(lines) > maxLines {
			resultLines = append(resultLines, fmt.Sprintf("%s   ...(truncated)%s", tertiaryColor, resetColor))
		}

		return "\n" + strings.Join(resultLines, "\n")
	}

	return ""
}

// formatWriteFileResult formats writeFile tool results with diff content
func formatWriteFileResult(result map[string]any, config *types.Config) string {
	if result == nil || len(result) == 0 {
		return ""
	}

	// Get theme colors
	theme := GetThemeForMode(config.Theme, config.OutputMode)
	tertiaryColor := ConvertColorToAnsi(theme.TextTertiary)
	resetColor := "\033[0m"

	// Get the basic result message (summary)
	var summary string
	if results, ok := result["results"].(string); ok && results != "" {
		summary = results
	}

	// Get the diff content
	var diffContent string
	if diff, ok := result["diff"].(string); ok && diff != "" {
		diffContent = diff
	}

	var output []string

	// Add the summary as the first line with L-shaped formatting
	if summary != "" {
		output = append(output, fmt.Sprintf("%s└─ %s%s", tertiaryColor, summary, resetColor))
	}

	// Handle diff content based on operation success
	success, _ := result["success"].(bool)
	if diffContent != "" {
		if success {
			// Operation was successful - show formatted diff
			var diffThemeName string
			if config.DiffTheme != "auto" {
				diffThemeName = config.DiffTheme
			} else {
				// Fall back to theme mapping if set to "auto"
				diffThemeName = GetDiffThemeForMainTheme(config.Theme)
			}
			
			diffTheme := GetDiffTheme(diffThemeName)
			formatter := NewDiffFormatter(diffTheme)
			formattedDiff := formatter.Format(diffContent)
			
			// Split diff into lines and add proper indentation
			diffLines := strings.Split(strings.TrimSpace(formattedDiff), "\n")
			for _, line := range diffLines {
				if strings.TrimSpace(line) != "" {
					// Add indentation to align with the L-shaped formatting
					output = append(output, fmt.Sprintf("   %s", line))
				}
			}
		} else {
			// Operation was cancelled or failed - show muted message
			output = append(output, fmt.Sprintf("%s   (diff preview was available but changes were not applied)%s", tertiaryColor, resetColor))
		}
	}

	if len(output) > 0 {
		return "\n" + strings.Join(output, "\n")
	}

	return ""
}