package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// OutputFormatter formats tool execution results from LLM responses
type OutputFormatter interface {
	// FormatResponse parses tool_outputs blocks and formats them using tool-specific formatters
	FormatResponse(response string) string
}

// DefaultOutputFormatter implements OutputFormatter using a tool registry
type DefaultOutputFormatter struct {
	registry Registry
}

// NewOutputFormatter creates a new output formatter with the given tool registry
func NewOutputFormatter(registry Registry) OutputFormatter {
	return &DefaultOutputFormatter{
		registry: registry,
	}
}

// FormatResponse parses and formats Gemini tool output blocks for better user experience
// Instead of removing tool outputs, it extracts and formats them in a user-friendly way
func (f *DefaultOutputFormatter) FormatResponse(response string) string {
	// Pattern to match tool_outputs blocks and capture the JSON content
	toolOutputPattern := regexp.MustCompile("(?s)```tool_outputs\\n(.*?)\\n```")
	
	// Find all tool output blocks
	matches := toolOutputPattern.FindAllStringSubmatch(response, -1)
	
	var formattedResults []string
	for _, match := range matches {
		if len(match) > 1 {
			jsonContent := match[1]
			if formatted := f.formatToolOutput(jsonContent); formatted != "" {
				formattedResults = append(formattedResults, formatted)
			}
		}
	}
	
	// Remove the raw tool_outputs blocks
	cleaned := toolOutputPattern.ReplaceAllString(response, "")
	
	// Insert formatted results at the beginning if we have any
	if len(formattedResults) > 0 {
		resultSection := strings.Join(formattedResults, "\n\n")
		// Add a separator between tool results and conversational text if both exist
		if strings.TrimSpace(cleaned) != "" {
			cleaned = resultSection + "\n\n" + cleaned
		} else {
			cleaned = resultSection
		}
	}
	
	// Clean up any remaining extra whitespace and normalize line breaks
	cleaned = regexp.MustCompile("\\n\\s*\\n\\s*\\n").ReplaceAllString(cleaned, "\n\n")
	cleaned = strings.TrimSpace(cleaned)
	
	// If the response becomes empty after cleaning AND we had tool outputs to process, provide a fallback
	if cleaned == "" && len(formattedResults) > 0 {
		return "I've processed your request."
	}
	
	return cleaned
}

// formatToolOutput formats a single tool output JSON string into user-friendly text
func (f *DefaultOutputFormatter) formatToolOutput(jsonContent string) string {
	// Parse the JSON
	var toolData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonContent), &toolData); err != nil {
		return "" // Skip malformed JSON
	}
	
	var results []string
	
	// Process each tool result in the JSON
	for key, value := range toolData {
		if !strings.HasSuffix(key, "_response") {
			continue
		}
		
		// Extract tool name (remove _response suffix)
		toolName := strings.TrimSuffix(key, "_response")
		
		// Parse the tool result
		resultData, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		
		// Look up the tool in the registry and use its formatter
		if tool, exists := f.registry.Get(toolName); exists {
			formatted := tool.FormatOutput(resultData)
			if formatted != "" {
				results = append(results, formatted)
			}
		} else {
			// Fallback to generic formatting if tool not found
			formatted := f.formatGenericOutput(toolName, resultData)
			if formatted != "" {
				results = append(results, formatted)
			}
		}
	}
	
	return strings.Join(results, "\n\n")
}

// formatGenericOutput formats tool output when the specific tool isn't found in registry
func (f *DefaultOutputFormatter) formatGenericOutput(toolName string, resultData map[string]interface{}) string {
	success, _ := resultData["success"].(bool)
	
	status := "[SUCCESS]"
	if !success {
		status = "[FAILED]"
	}
	
	// Try to find the main output field
	var mainOutput string
	for _, field := range []string{"output", "content", "files", "results", "matches", "status", "message", "result"} {
		if output, ok := resultData[field].(string); ok && output != "" {
			mainOutput = output
			break
		}
	}
	
	// Convert camelCase to title case for display
	displayName := formatToolNameForDisplay(toolName)
	
	if mainOutput == "" {
		return fmt.Sprintf("%s **%s completed**", status, displayName)
	}
	
	// Format with code block if it looks like structured output
	if strings.Contains(mainOutput, "\n") || len(mainOutput) > 50 {
		return fmt.Sprintf("%s **%s**\n```\n%s\n```", status, displayName, strings.TrimSpace(mainOutput))
	}
	
	return fmt.Sprintf("%s **%s**: %s", status, displayName, mainOutput)
}

// formatToolNameForDisplay converts technical tool names to user-friendly display names
func formatToolNameForDisplay(toolName string) string {
	switch toolName {
	case "runBashCommand":
		return "Command Output"
	case "listFiles":
		return "File Listing"
	case "readFile", "catFile":
		return "File Content"
	case "writeFile":
		return "File Written"
	case "searchInFiles", "grepFiles":
		return "Search Results"
	case "findFiles":
		return "Find Results"
	case "gitStatus":
		return "Git Status"
	default:
		// Convert camelCase to title case as fallback
		result := ""
		for i, char := range toolName {
			if i > 0 && char >= 'A' && char <= 'Z' {
				result += " "
			}
			if i == 0 {
				result += strings.ToUpper(string(char))
			} else {
				result += string(char)
			}
		}
		return result
	}
}