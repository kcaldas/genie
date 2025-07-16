package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputFormatter_FormatResponse(t *testing.T) {
	// Create a formatter without registry dependency
	formatter := NewOutputFormatter(nil)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bash_command_success",
			input:    "```tool_outputs\n{\"bash_response\": {\"results\": \"/Users/kcaldas/dev/genie\\n\", \"success\": true}}\n```\n/Users/kcaldas/dev/genie",
			expected: "bash - Success\n\n/Users/kcaldas/dev/genie",
		},
		{
			name:     "bash_command_failure",
			input:    "```tool_outputs\n{\"bash_response\": {\"results\": \"\", \"success\": false, \"error\": \"command not found\"}}\n```\nSorry, that command failed.",
			expected: "bash - Failure\n\nSorry, that command failed.",
		},
		{
			name:     "file_listing_success",
			input:    "```tool_outputs\n{\"listFiles_response\": {\"results\": \"cmd/\\nmain.go\", \"success\": true}}\n```\nHere are the files in your project.",
			expected: "listFiles - Success\n\nHere are the files in your project.",
		},
		{
			name:     "no_tool_outputs_unchanged",
			input:    "This is a normal response without tool outputs.",
			expected: "This is a normal response without tool outputs.",
		},
		{
			name:     "multiple_tool_outputs",
			input:    "```tool_outputs\n{\"bash_response\": {\"results\": \"hello\", \"success\": true}}\n```\nCommand executed.\n```tool_outputs\n{\"listFiles_response\": {\"results\": \"file1.txt\", \"success\": true}}\n```\nFiles listed.",
			expected: "bash - Success\n\nlistFiles - Success\n\nCommand executed.\n\nFiles listed.",
		},
		{
			name:     "only_tool_outputs_fallback",
			input:    "```tool_outputs\n{\"bash_response\": {\"results\": \"\", \"success\": true}}\n```",
			expected: "bash - Success",
		},
		{
			name:     "unknown_tool_generic_formatting",
			input:    "```tool_outputs\n{\"unknownTool_response\": {\"result\": \"some output\", \"success\": true}}\n```\nProcessed successfully.",
			expected: "unknownTool - Success\n\nProcessed successfully.",
		},
		{
			name:     "text_before_tool_output",
			input:    "Let me check that for you:\n```tool_outputs\n{\"bash_response\": {\"results\": \"hello world\", \"success\": true}}\n```\nThat's the output.",
			expected: "bash - Success\n\nLet me check that for you:\n\nThat's the output.",
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
	formatter := NewOutputFormatter(nil)

	result := formatter.FormatResponse("")
	// Empty input should remain empty (no tool outputs to process)
	assert.Equal(t, "", result)
}

func TestOutputFormatter_MalformedJSON(t *testing.T) {
	formatter := NewOutputFormatter(nil)

	input := "```tool_outputs\n{malformed json}\n```\nSome text after."
	result := formatter.FormatResponse(input)
	
	// Should skip malformed JSON and just clean up the text
	assert.Equal(t, "Some text after.", result)
	assert.False(t, strings.Contains(result, "```tool_outputs"))
}
