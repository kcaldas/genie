package lmstudio

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
	assert.Equal(t, "assistant", messages[2].Role)
	assert.Equal(t, "File: a.md\n\n[Memory]\n\nq2", messages[3].Content.Parts[0].Text)
}

func TestBuildMessages_HistoryToolCallsAreNative(t *testing.T) {
	messages := (&Client{}).buildMessages(ai.Prompt{History: []ai.HistoryTurn{{User: "q1", Actions: []ai.HistoryAction{{Tool: "bash", Args: `command="go test"`, Summary: "Failed: TestX", Failed: true}}, Assistant: "a1"}}, Text: "q2"})

	require.Len(t, messages, 5)
	assert.Equal(t, "assistant", messages[1].Role)
	require.Len(t, messages[1].ToolCalls, 1)
	assert.Equal(t, "hist_0_0", messages[1].ToolCalls[0].ID)
	assert.Equal(t, "bash", messages[1].ToolCalls[0].Function.Name)
	assert.Equal(t, `"{\"command\":\"go test\"}"`, string(messages[1].ToolCalls[0].Function.Arguments), "arguments ride as a JSON string on the OpenAI-compatible wire")
	assert.Equal(t, "tool", messages[2].Role)
	assert.Equal(t, "hist_0_0", messages[2].ToolCallID)
	assert.Equal(t, "Failed: TestX", messages[2].Content.Parts[0].Text)
	assert.Equal(t, "a1", messages[3].Content.Parts[0].Text)
}
