package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// OutputFormatter formats tool execution results from LLM responses
type OutputFormatter interface {
	// FormatResponse parses tool_outputs blocks and formats them as simple status messages
	FormatResponse(response string) string
}

// DefaultOutputFormatter implements OutputFormatter with simple formatting
type DefaultOutputFormatter struct {
}

// NewOutputFormatter creates a new output formatter
func NewOutputFormatter(registry Registry) OutputFormatter {
	return &DefaultOutputFormatter{}
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

// formatToolOutput formats a single tool output JSON string into a simple status message
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
		
		// Extract success status
		success, _ := resultData["success"].(bool)
		status := "Success"
		if !success {
			status = "Failure"
		}
		
		// Format as simple status message
		results = append(results, fmt.Sprintf("%s - %s", toolName, status))
	}
	
	return strings.Join(results, "\n\n")
}

