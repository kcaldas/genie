package tools

import (
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestOutputFormatter_FormatResponse(t *testing.T) {
	// Create a test registry with a bash tool
	eventBus := events.NewEventBus()
	registry := NewDefaultRegistry(eventBus)
	formatter := NewOutputFormatter(registry)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bash_command_success",
			input:    "```tool_outputs\n{\"runBashCommand_response\": {\"results\": \"/Users/kcaldas/dev/genie\\n\", \"success\": true}}\n```\n/Users/kcaldas/dev/genie",
			expected: "**Command Output**\n```\n/Users/kcaldas/dev/genie\n```\n\n/Users/kcaldas/dev/genie",
		},
		{
			name:     "bash_command_failure",
			input:    "```tool_outputs\n{\"runBashCommand_response\": {\"results\": \"\", \"success\": false, \"error\": \"command not found\"}}\n```\nSorry, that command failed.",
			expected: "**Command Failed**\n```\ncommand not found\n```\n\nSorry, that command failed.",
		},
		{
			name:     "file_listing_success",
			input:    "```tool_outputs\n{\"listFiles_response\": {\"results\": \"cmd/\\nmain.go\", \"success\": true}}\n```\nHere are the files in your project.",
			expected: "**Files in Directory**\n[DIR]  cmd/\n[FILE] main.go\n\nHere are the files in your project.",
		},
		{
			name:     "no_tool_outputs_unchanged",
			input:    "This is a normal response without tool outputs.",
			expected: "This is a normal response without tool outputs.",
		},
		{
			name:     "multiple_tool_outputs",
			input:    "```tool_outputs\n{\"runBashCommand_response\": {\"results\": \"hello\", \"success\": true}}\n```\nCommand executed.\n```tool_outputs\n{\"listFiles_response\": {\"results\": \"file1.txt\", \"success\": true}}\n```\nFiles listed.",
			expected: "**Command Output**\n```\nhello\n```\n\n**Files in Directory**\n[FILE] file1.txt\n\nCommand executed.\n\nFiles listed.",
		},
		{
			name:     "only_tool_outputs_fallback",
			input:    "```tool_outputs\n{\"runBashCommand_response\": {\"results\": \"\", \"success\": true}}\n```",
			expected: "**Command completed successfully**",
		},
		{
			name:     "unknown_tool_generic_formatting",
			input:    "```tool_outputs\n{\"unknownTool_response\": {\"result\": \"some output\", \"success\": true}}\n```\nProcessed successfully.",
			expected: "[SUCCESS] **Unknown Tool**: some output\n\nProcessed successfully.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatter.FormatResponse(tc.input)
			assert.Equal(t, tc.expected, result)
			
			// Verify no raw tool outputs remain
			assert.False(t, strings.Contains(result, "```tool_outputs"), 
				"Result should not contain raw tool_outputs blocks")
			assert.False(t, strings.Contains(result, "_response"), 
				"Result should not contain raw tool response JSON")
		})
	}
}

func TestOutputFormatter_EmptyInput(t *testing.T) {
	eventBus := events.NewEventBus()
	registry := NewDefaultRegistry(eventBus)
	formatter := NewOutputFormatter(registry)

	result := formatter.FormatResponse("")
	// Empty input should remain empty (no tool outputs to process)
	assert.Equal(t, "", result)
}

func TestOutputFormatter_MalformedJSON(t *testing.T) {
	eventBus := events.NewEventBus()
	registry := NewDefaultRegistry(eventBus)
	formatter := NewOutputFormatter(registry)

	input := "```tool_outputs\n{malformed json}\n```\nSome text after."
	result := formatter.FormatResponse(input)
	
	// Should skip malformed JSON and just clean up the text
	assert.Equal(t, "Some text after.", result)
	assert.False(t, strings.Contains(result, "```tool_outputs"))
}

func TestFormatToolNameForDisplay(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"runBashCommand", "Command Output"},
		{"listFiles", "File Listing"},
		{"readFile", "File Content"},
		{"writeFile", "File Written"},
		{"searchInFiles", "Search Results"},
		{"findFiles", "Find Results"},
		{"gitStatus", "Git Status"},
		{"unknownTool", "Unknown Tool"},
		{"camelCaseExample", "Camel Case Example"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := formatToolNameForDisplay(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOutputFormatter_IntegrationWithRealTools(t *testing.T) {
	// Test that the formatter works correctly with real tool instances
	eventBus := events.NewEventBus()
	registry := NewDefaultRegistry(eventBus)
	formatter := NewOutputFormatter(registry)

	// Test bash tool formatting
	bashResult := map[string]interface{}{
		"success": true,
		"results":  "test output",
	}
	
	// Get the bash tool from registry and test its FormatOutput method
	bashTool, exists := registry.Get("runBashCommand")
	assert.True(t, exists, "Bash tool should be in registry")
	
	formatted := bashTool.FormatOutput(bashResult)
	expected := "**Command Output**\n```\ntest output\n```"
	assert.Equal(t, expected, formatted)

	// Test that the formatter correctly uses the tool's FormatOutput method
	geminiResponse := "```tool_outputs\n{\"runBashCommand_response\": {\"results\": \"test output\", \"success\": true}}\n```\nThe command was executed."
	formatterResult := formatter.FormatResponse(geminiResponse)
	
	assert.Contains(t, formatterResult, "**Command Output**")
	assert.Contains(t, formatterResult, "test output")
	assert.Contains(t, formatterResult, "The command was executed.")
	assert.NotContains(t, formatterResult, "```tool_outputs")
}