package openai

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMessages_NativeHistoryLayout(t *testing.T) {
	prompt := ai.Prompt{
		Instruction: "be kind",
		Context:     ai.TurnContext{Project: "# AGENTS.md", Files: "File: a.md", Host: "[Memory]"},
		History:     []ai.HistoryTurn{{User: "q1", Assistant: "a1"}},
		Text:        "q2",
	}
	_, tokenMessages, err := (&Client{}).buildMessages(prompt)
	require.NoError(t, err)

	require.Len(t, tokenMessages, 4)
	assert.Equal(t, tokenMessage{Role: "system", Content: "be kind\n\n# AGENTS.md"}, tokenMessages[0])
	assert.Equal(t, tokenMessage{Role: "user", Content: "q1"}, tokenMessages[1])
	assert.Equal(t, tokenMessage{Role: "assistant", Content: "a1"}, tokenMessages[2])
	assert.Equal(t, tokenMessage{Role: "user", Content: "File: a.md\n\n[Memory]\n\nq2"}, tokenMessages[3])
}

func TestBuildResponseInitialInput_NativeHistoryLayout(t *testing.T) {
	prompt := ai.Prompt{
		Instruction: "be kind",
		Context:     ai.TurnContext{Project: "# AGENTS.md", Files: "File: a.md", Host: "[Memory]"},
		History:     []ai.HistoryTurn{{User: "q1", Assistant: "a1"}},
		Text:        "q2",
	}
	input := (&Client{}).buildResponseInitialInput(prompt)

	require.Len(t, input, 3)
	require.NotNil(t, input[0].OfMessage)
	assert.Equal(t, "user", string(input[0].OfMessage.Role))
	assert.Equal(t, "q1", input[0].OfMessage.Content.OfString.Value)
	require.NotNil(t, input[1].OfMessage)
	assert.Equal(t, "assistant", string(input[1].OfMessage.Role))
	assert.Equal(t, "a1", input[1].OfMessage.Content.OfString.Value)
	require.NotNil(t, input[2].OfInputMessage)
	assert.Equal(t, "File: a.md\n\n[Memory]\n\nq2", input[2].OfInputMessage.Content[0].OfInputText.Text)
}
