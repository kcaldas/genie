package deepseek

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMessages_NativeHistoryLayout(t *testing.T) {
	messages, err := (&Client{}).buildMessages(ai.Prompt{
		Instruction: "be kind",
		Context:     ai.TurnContext{Project: "# AGENTS.md", Files: "File: a.md", Host: "[Memory]"},
		History:     []ai.HistoryTurn{{User: "q1", Assistant: "a1"}},
		Text:        "q2",
	}, "deepseek-chat")
	require.NoError(t, err)

	require.Len(t, messages, 4)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "be kind\n\n# AGENTS.md", messages[0].Content.Parts[0].Text)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "assistant", messages[2].Role)
	assert.Equal(t, "File: a.md\n\n[Memory]\n\nq2", messages[3].Content.Parts[0].Text)
}

func TestBuildMessages_SchemaOnlyPromptGetsASystemMessage(t *testing.T) {
	schema := &ai.Schema{Type: ai.TypeObject, Properties: map[string]*ai.Schema{"ok": {Type: ai.TypeBoolean}}}

	messages, err := (&Client{}).buildMessages(ai.Prompt{Text: "hi", ResponseSchema: schema}, "deepseek-chat")
	require.NoError(t, err)

	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Contains(t, messages[0].Content.Parts[0].Text, "You must respond with JSON matching this schema")
	assert.Contains(t, messages[0].Content.Parts[0].Text, `"ok"`)
	assert.Equal(t, "user", messages[1].Role)
}
