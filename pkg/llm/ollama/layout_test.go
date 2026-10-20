package ollama

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMessages_NativeHistoryLayout(t *testing.T) {
	messages := (&Client{}).buildMessages(ai.Prompt{
		Instruction: "be kind",
		Context:     ai.TurnContext{Project: "# AGENTS.md", Files: "File: a.md", Host: "[Memory]"},
		History:     []ai.HistoryTurn{{User: "q1", Assistant: "a1"}},
		Text:        "q2",
	})

	require.Len(t, messages, 4)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "be kind\n\n# AGENTS.md", messages[0].Content.Parts[0].Text)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "q1", messages[1].Content.Parts[0].Text)
	assert.Equal(t, "assistant", messages[2].Role)
	assert.Equal(t, "a1", messages[2].Content.Parts[0].Text)
	assert.Equal(t, "user", messages[3].Role)
	assert.Equal(t, "File: a.md\n\n[Memory]\n\nq2", messages[3].Content.Parts[0].Text)
}

// Past tool calls replay as tool_calls on an assistant message followed by
// tool-role results, never as text on the user side.
func TestBuildMessages_HistoryToolCallsAreNative(t *testing.T) {
	messages := (&Client{}).buildMessages(ai.Prompt{History: []ai.HistoryTurn{{User: "q1", Actions: []ai.HistoryAction{{Tool: "bash", Args: `command="go test"`, Summary: "Failed: TestX", Failed: true}}, Assistant: "a1"}}, Text: "q2"})

	require.Len(t, messages, 6)
	assert.Equal(t, "q1", messages[1].Content.Parts[0].Text)
	assert.Equal(t, "assistant", messages[2].Role)
	require.Len(t, messages[2].ToolCalls, 1)
	assert.Equal(t, "hist_0_0", messages[2].ToolCalls[0].ID)
	assert.Equal(t, "bash", messages[2].ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"command":"go test"}`, string(messages[2].ToolCalls[0].Function.Arguments), "Ollama's own API takes arguments as an object")
	assert.Equal(t, "tool", messages[3].Role)
	assert.Equal(t, "hist_0_0", messages[3].ToolCallID)
	assert.Equal(t, "bash", messages[3].ToolName)
	assert.Equal(t, "[recorded] Failed: TestX", messages[3].Content.Parts[0].Text)
	assert.Equal(t, "assistant", messages[4].Role)
	assert.Equal(t, "a1", messages[4].Content.Parts[0].Text)
	assert.Equal(t, "q2", messages[5].Content.Parts[0].Text)
}
