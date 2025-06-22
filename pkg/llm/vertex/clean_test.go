package vertex

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanGeminiToolOutputs(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean_tool_outputs_block",
			input:    "```tool_outputs\n{\"runBashCommand_response\": {\"output\": \"/Users/kcaldas/dev/genie\\n\", \"success\": true}}\n```\n/Users/kcaldas/dev/genie",
			expected: "/Users/kcaldas/dev/genie",
		},
		{
			name:     "clean_listfiles_tool_outputs",
			input:    "```tool_outputs\n{\"listFiles_response\": {\"files\": \"cmd/\\nmain.go\", \"success\": true}}\n```\nHere are the files in your project.",
			expected: "Here are the files in your project.",
		},
		{
			name:     "no_tool_outputs_unchanged",
			input:    "This is a normal response without tool outputs.",
			expected: "This is a normal response without tool outputs.",
		},
		{
			name:     "multiple_tool_outputs_blocks",
			input:    "```tool_outputs\n{\"tool1\": \"result1\"}\n```\nSome text\n```tool_outputs\n{\"tool2\": \"result2\"}\n```\nFinal response.",
			expected: "Some textFinal response.",
		},
		{
			name:     "only_tool_outputs_fallback",
			input:    "```tool_outputs\n{\"onlyTool\": \"result\"}\n```",
			expected: "I've processed your request.",
		},
		{
			name:     "real_captured_response",
			input:    "```tool_outputs\n{\"runBashCommand_response\": {\"output\": \"/Users/kcaldas/dev/genie\\n\", \"success\": true}}\n```\n/Users/kcaldas/dev/genie\n",
			expected: "/Users/kcaldas/dev/genie",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := cleanGeminiToolOutputs(tc.input)
			assert.Equal(t, tc.expected, result)
			
			// Verify no tool outputs remain
			assert.False(t, strings.Contains(result, "```tool_outputs"), 
				"Result should not contain tool_outputs blocks")
			assert.False(t, strings.Contains(result, "_response"), 
				"Result should not contain tool response JSON")
		})
	}
}

func TestVertexCleaningIntegration(t *testing.T) {
	// Test the integration of cleaning with vertex responses
	
	// Simulate the problematic Gemini response format
	geminiResponse := "```tool_outputs\n{\"listFiles_response\": {\"files\": \"cmd/\\ngo.mod\\nREADME.md\", \"success\": true}}\n```\nI can see this is a Go project with the following structure:\n\n- **cmd/** - Application entry points\n- **go.mod** - Go module definition\n- **README.md** - Project documentation"
	
	cleaned := cleanGeminiToolOutputs(geminiResponse)
	
	expected := "I can see this is a Go project with the following structure:\n- **cmd/** - Application entry points\n- **go.mod** - Go module definition\n- **README.md** - Project documentation"
	
	assert.Equal(t, expected, cleaned)
	
	// Verify the cleaning worked
	assert.False(t, strings.Contains(cleaned, "```tool_outputs"))
	assert.False(t, strings.Contains(cleaned, "listFiles_response"))
	assert.False(t, strings.Contains(cleaned, "\"success\": true"))
	
	// Verify useful content remains
	assert.True(t, strings.Contains(cleaned, "Go project"))
	assert.True(t, strings.Contains(cleaned, "cmd/"))
	assert.True(t, strings.Contains(cleaned, "structure"))
	
	t.Logf("✅ Gemini tool output cleaning successful")
	t.Logf("   Original length: %d chars", len(geminiResponse))
	t.Logf("   Cleaned length: %d chars", len(cleaned))
	t.Logf("   Removed: tool_outputs blocks and JSON")
	t.Logf("   Preserved: conversational response")
}