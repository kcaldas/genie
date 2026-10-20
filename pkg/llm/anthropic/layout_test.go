package anthropic

import (
	"encoding/json"
	"testing"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
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
	switch {
	case b.OfText != nil:
		return b.OfText.CacheControl.TTL != ""
	case b.OfToolUse != nil:
		return b.OfToolUse.CacheControl.TTL != ""
	case b.OfToolResult != nil:
		return b.OfToolResult.CacheControl.TTL != ""
	}
	return false
}

func TestBuildSystemBlocks_OneStableBlockWithMarker(t *testing.T) {
	blocks := layoutClient(t).buildSystemBlocks(layoutPrompt())

	require.Len(t, blocks, 1)
	assert.Equal(t, "be kind\n\n# AGENTS.md\n\n"+llmshared.RecordedNote, blocks[0].Text)
	assert.NotEmpty(t, blocks[0].CacheControl.TTL, "the stable system block carries the cache marker")
}

func TestBuildMessages_OneMessagePerSideThenTail(t *testing.T) {
	messages, err := layoutClient(t).buildMessages(layoutPrompt())
	require.NoError(t, err)

	require.Len(t, messages, 7)
	assert.Equal(t, anthropic_sdk.MessageParamRoleUser, messages[0].Role)
	assert.Equal(t, "q1", blockText(messages[0].Content[0]))
	assert.Equal(t, anthropic_sdk.MessageParamRoleAssistant, messages[1].Role)
	assert.Equal(t, "a1", blockText(messages[1].Content[0]))
	assert.Equal(t, "q2", blockText(messages[2].Content[0]), "the user's own words only")

	calls := messages[3]
	assert.Equal(t, anthropic_sdk.MessageParamRoleAssistant, calls.Role)
	require.Len(t, calls.Content, 1)
	require.NotNil(t, calls.Content[0].OfToolUse, "the digest replays as a tool_use block")
	assert.Equal(t, "hist_1_0", calls.Content[0].OfToolUse.ID)
	assert.Equal(t, "readFile", calls.Content[0].OfToolUse.Name)
	assert.JSONEq(t, `{"args":"a.md"}`, string(calls.Content[0].OfToolUse.Input.(json.RawMessage)))

	results := messages[4]
	assert.Equal(t, anthropic_sdk.MessageParamRoleUser, results.Role)
	require.Len(t, results.Content, 1)
	require.NotNil(t, results.Content[0].OfToolResult)
	assert.Equal(t, "hist_1_0", results.Content[0].OfToolResult.ToolUseID)
	assert.Equal(t, "[recorded] (no summary)", results.Content[0].OfToolResult.Content[0].OfText.Text)

	assert.Equal(t, "a2", blockText(messages[5].Content[0]), "the reply text only")

	tail := messages[6]
	assert.Equal(t, anthropic_sdk.MessageParamRoleUser, tail.Role)
	require.Len(t, tail.Content, 2)
	assert.Equal(t, "File: a.md\n\n[Memory]\n\nUser: q3", blockText(tail.Content[0]))
	assert.NotNil(t, tail.Content[1].OfImage, "image rides in the tail after the text")
}

func TestBuildMessages_MarkerOnLastHistoryMessageOnly(t *testing.T) {
	messages, err := layoutClient(t).buildMessages(layoutPrompt())
	require.NoError(t, err)

	for i, m := range messages[:5] {
		assert.False(t, hasMarker(m.Content[0]), "message %d must not carry a marker", i)
	}
	assert.True(t, hasMarker(messages[5].Content[0]), "the last history message closes the cached prefix")
	assert.False(t, hasMarker(messages[6].Content[0]), "the tail changes every turn and is never marked")
}

// Parallel calls in one turn share one tool_use message and one
// tool_result message, the shape the API requires; a failed call is an
// error result.
func TestBuildMessages_ParallelCallsShareOneResultMessage(t *testing.T) {
	prompt := ai.Prompt{History: []ai.HistoryTurn{{User: "q", Actions: []ai.HistoryAction{
		{Tool: "bash", Args: `command="go test"`, Summary: "Failed: TestX", Failed: true},
		{Tool: "readFile", Args: `path="a.go"`, Summary: "read a.go"},
	}, Assistant: "a"}}, Text: "n"}
	messages, err := layoutClient(t).buildMessages(prompt)
	require.NoError(t, err)

	require.Len(t, messages, 5)
	require.Len(t, messages[1].Content, 2, "both calls on one assistant message")
	results := messages[2]
	require.Len(t, results.Content, 2, "both results on one user message")
	assert.True(t, results.Content[0].OfToolResult.IsError.Value)
	assert.Equal(t, "hist_0_1", results.Content[1].OfToolResult.ToolUseID)
	assert.False(t, results.Content[1].OfToolResult.IsError.Value)
}

// A turn can end on its tool results (no reply text). The cache marker
// then lands on the tool_result block instead of a text block.
func TestBuildMessages_MarkerOnToolResultWhenTheTurnHasNoReply(t *testing.T) {
	prompt := ai.Prompt{History: []ai.HistoryTurn{{User: "q", Actions: []ai.HistoryAction{{Tool: "bash", Summary: "ok"}}}}, Text: "n"}
	messages, err := layoutClient(t).buildMessages(prompt)
	require.NoError(t, err)

	require.Len(t, messages, 4)
	assert.True(t, hasMarker(messages[2].Content[0]))
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

// Results group per turn only: a second turn with its own calls starts a
// new tool_use / tool_result pair after its user message.
func TestBuildMessages_ResultsNeverMergeAcrossTurns(t *testing.T) {
	prompt := ai.Prompt{History: []ai.HistoryTurn{
		{User: "q1", Actions: []ai.HistoryAction{{Tool: "bash", Summary: "ok"}}},
		{User: "q2", Actions: []ai.HistoryAction{{Tool: "readFile", Summary: "read"}}, Assistant: "a2"},
	}, Text: "q3"}
	messages, err := layoutClient(t).buildMessages(prompt)
	require.NoError(t, err)

	require.Len(t, messages, 8)
	assert.Equal(t, "hist_0_0", messages[2].Content[0].OfToolResult.ToolUseID)
	assert.Equal(t, "q2", blockText(messages[3].Content[0]))
	assert.Equal(t, "hist_1_0", messages[4].Content[0].OfToolUse.ID)
	assert.Equal(t, "hist_1_0", messages[5].Content[0].OfToolResult.ToolUseID)
	require.Len(t, messages[5].Content, 1)
}
