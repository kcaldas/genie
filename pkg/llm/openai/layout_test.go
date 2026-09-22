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

func boundaryPrompt(model string) ai.Prompt {
	return ai.Prompt{
		ModelName:   model,
		Instruction: "be kind",
		Context:     ai.TurnContext{Files: "File: a.md", Host: "[Memory]"},
		History:     []ai.HistoryTurn{{User: "q1", Assistant: "a1"}},
		Message:     "q2",
		Text:        "# CURRENT MESSAGE\nUser: q2",
		Images:      []*ai.Image{{Type: "image/png", Data: []byte{1}}},
	}
}

// GPT-5.6-class models look the cache up only at user-message endings, so
// the final user message must be the bare message that the next turn will
// replay as history, and the volatile context must follow it.
func TestBuildResponseInitialInput_MessageBoundaryModelsPutContextAfterTheMessage(t *testing.T) {
	input := (&Client{}).buildResponseInitialInput(boundaryPrompt("gpt-5.6-luna"))

	require.Len(t, input, 4)
	assert.Equal(t, "q1", input[0].OfMessage.Content.OfString.Value)
	assert.Equal(t, "a1", input[1].OfMessage.Content.OfString.Value)

	user := input[2].OfInputMessage
	require.NotNil(t, user)
	assert.Equal(t, "user", user.Role)
	assert.Equal(t, "q2", user.Content[0].OfInputText.Text, "bare message, no wrapper, no context")
	require.Len(t, user.Content, 2)
	require.NotNil(t, user.Content[1].OfInputImage, "images stay on the user message")

	context := input[3].OfMessage
	require.NotNil(t, context)
	assert.Equal(t, "developer", string(context.Role))
	assert.Equal(t, "File: a.md\n\n[Memory]", context.Content.OfString.Value)
}

func TestBuildResponseInitialInput_NoContextItemWhenContextIsEmpty(t *testing.T) {
	prompt := boundaryPrompt("gpt-5.6-luna")
	prompt.Context = ai.TurnContext{}

	input := (&Client{}).buildResponseInitialInput(prompt)

	require.Len(t, input, 3)
	assert.Equal(t, "q2", input[2].OfInputMessage.Content[0].OfInputText.Text)
}

func TestBuildResponseInitialInput_FallsBackToTextWithoutARawMessage(t *testing.T) {
	prompt := boundaryPrompt("gpt-5.6-luna")
	prompt.Message = ""

	input := (&Client{}).buildResponseInitialInput(prompt)

	assert.Equal(t, "# CURRENT MESSAGE\nUser: q2", input[2].OfInputMessage.Content[0].OfInputText.Text)
}

func TestBuildResponseInitialInput_IntervalCachedModelsKeepContextInTheTail(t *testing.T) {
	input := (&Client{}).buildResponseInitialInput(boundaryPrompt("gpt-5.4-mini"))

	require.Len(t, input, 3)
	assert.Equal(t, "File: a.md\n\n[Memory]\n\n# CURRENT MESSAGE\nUser: q2", input[2].OfInputMessage.Content[0].OfInputText.Text)
}

func TestUsesMessageBoundaryCache(t *testing.T) {
	for model, want := range map[string]bool{"gpt-5.6-luna": true, "gpt-5.6": true, "gpt-5.7-sol": true, "gpt-6": true, "gpt-5.4-mini": false, "gpt-5.5": false, "o3-mini": false, "gpt-4o": false} {
		assert.Equal(t, want, usesMessageBoundaryCache(model), model)
	}
}
