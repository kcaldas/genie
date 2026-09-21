package anthropic

import (
	"testing"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func layoutClient(t *testing.T) *Client {
	t.Helper()
	raw, err := NewClient(&events.NoOpEventBus{}, WithMessageClient(&mockMessageClient{t: t}))
	require.NoError(t, err)
	return raw.(*Client)
}

func layoutPrompt() ai.Prompt {
	return ai.Prompt{
		Instruction: "be kind",
		Context:     ai.TurnContext{Project: "# AGENTS.md", Files: "File: a.md", Host: "[Memory]"},
		History: []ai.HistoryTurn{
			{User: "q1", Assistant: "a1"},
			{User: "q2", Actions: []ai.HistoryAction{{Tool: "readFile", Args: "a.md"}}, Assistant: "a2"},
		},
		Text:   "User: q3",
		Images: []*ai.Image{{Type: "image/png", Data: []byte{1, 2}}},
	}
}

func blockText(b anthropic_sdk.ContentBlockParamUnion) string {
	if b.OfText != nil {
		return b.OfText.Text
	}
	return ""
}

func hasMarker(b anthropic_sdk.ContentBlockParamUnion) bool {
	return b.OfText != nil && b.OfText.CacheControl.TTL != ""
}

func TestBuildSystemBlocks_OneStableBlockWithMarker(t *testing.T) {
	blocks := layoutClient(t).buildSystemBlocks(layoutPrompt())

	require.Len(t, blocks, 1)
	assert.Equal(t, "be kind\n\n# AGENTS.md", blocks[0].Text)
	assert.NotEmpty(t, blocks[0].CacheControl.TTL, "the stable system block carries the cache marker")
}

func TestBuildMessages_OneMessagePerSideThenTail(t *testing.T) {
	messages, err := layoutClient(t).buildMessages(layoutPrompt())
	require.NoError(t, err)

	require.Len(t, messages, 5)
	assert.Equal(t, anthropic_sdk.MessageParamRoleUser, messages[0].Role)
	assert.Equal(t, "q1", blockText(messages[0].Content[0]))
	assert.Equal(t, anthropic_sdk.MessageParamRoleAssistant, messages[1].Role)
	assert.Equal(t, "a1", blockText(messages[1].Content[0]))
	assert.Equal(t, "q2", blockText(messages[2].Content[0]))
	assert.Equal(t, "Assistant Actions:\n- readFile a.md\n\na2", blockText(messages[3].Content[0]))

	tail := messages[4]
	assert.Equal(t, anthropic_sdk.MessageParamRoleUser, tail.Role)
	require.Len(t, tail.Content, 2)
	assert.Equal(t, "File: a.md\n\n[Memory]\n\nUser: q3", blockText(tail.Content[0]))
	assert.NotNil(t, tail.Content[1].OfImage, "image rides in the tail after the text")
}

func TestBuildMessages_MarkerOnLastHistoryMessageOnly(t *testing.T) {
	messages, err := layoutClient(t).buildMessages(layoutPrompt())
	require.NoError(t, err)

	for i, m := range messages[:3] {
		assert.False(t, hasMarker(m.Content[0]), "message %d must not carry a marker", i)
	}
	assert.True(t, hasMarker(messages[3].Content[0]), "the last history message closes the cached prefix")
	assert.False(t, hasMarker(messages[4].Content[0]), "the tail changes every turn and is never marked")
}

func TestBuildMessages_NoHistoryIsJustTheTail(t *testing.T) {
	messages, err := layoutClient(t).buildMessages(ai.Prompt{Instruction: "be kind", Text: "hi"})
	require.NoError(t, err)

	require.Len(t, messages, 1)
	assert.Equal(t, "hi", blockText(messages[0].Content[0]))
	assert.False(t, hasMarker(messages[0].Content[0]))
}

func TestBuildMessages_DisableCacheDropsAllMarkers(t *testing.T) {
	prompt := layoutPrompt()
	prompt.DisableCache = true
	client := layoutClient(t)

	messages, err := client.buildMessages(prompt)
	require.NoError(t, err)
	for i, m := range messages {
		assert.False(t, hasMarker(m.Content[0]), "message %d", i)
	}
	assert.Empty(t, client.buildSystemBlocks(prompt)[0].CacheControl.TTL)
}

func TestBuildMessages_SkipsEmptySidesOfATurn(t *testing.T) {
	prompt := ai.Prompt{History: []ai.HistoryTurn{{Assistant: "seeded"}, {User: "q"}}, Text: "hi"}

	messages, err := layoutClient(t).buildMessages(prompt)
	require.NoError(t, err)

	require.Len(t, messages, 3)
	assert.Equal(t, anthropic_sdk.MessageParamRoleAssistant, messages[0].Role)
	assert.Equal(t, anthropic_sdk.MessageParamRoleUser, messages[1].Role)
}
