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
