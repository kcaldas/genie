package shared

import (
	"encoding/json"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLayoutConversation_SystemIsInstructionThenProject(t *testing.T) {
	c := LayoutConversation(ai.Prompt{Instruction: "be kind", Context: ai.TurnContext{Project: "# AGENTS.md"}})

	assert.Equal(t, "be kind\n\n# AGENTS.md", c.System)
}

func TestLayoutConversation_TailHoldsVolatileContextInOrder(t *testing.T) {
	c := LayoutConversation(ai.Prompt{
		Context: ai.TurnContext{Files: "File: a.md", Skill: "# skill", Host: "[Memory]", Tasks: "- todo"},
		Text:    "User: hi",
	})

	assert.Equal(t, "File: a.md\n\n# skill\n\n[Memory]\n\n- todo", c.Context)
	assert.Equal(t, "User: hi", c.Text)
	assert.Equal(t, "File: a.md\n\n# skill\n\n[Memory]\n\n- todo\n\nUser: hi", c.TailText())
}

func TestLayoutConversation_SkipsEmptyBlocks(t *testing.T) {
	c := LayoutConversation(ai.Prompt{Instruction: " be kind ", Context: ai.TurnContext{Host: "[Memory]"}, Text: "hi"})

	assert.Equal(t, "be kind", c.System)
	assert.Equal(t, "[Memory]", c.Context)
	assert.Equal(t, "[Memory]\n\nhi", c.TailText())
}

func TestLayoutConversation_TailTextWithoutContextIsJustTheMessage(t *testing.T) {
	c := LayoutConversation(ai.Prompt{Text: "hi"})

	assert.Equal(t, "hi", c.TailText())
}

func TestLayoutConversation_CarriesHistoryAndImages(t *testing.T) {
	img := &ai.Image{Type: "image/png", Data: []byte{1}}
	turns := []ai.HistoryTurn{{User: "q", Assistant: "a"}}
	c := LayoutConversation(ai.Prompt{History: turns, Images: []*ai.Image{img}})

	assert.Equal(t, turns, c.Turns)
	assert.Equal(t, []*ai.Image{img}, c.Images)
}

// A past turn's tool calls replay as what they were: tool calls on an
// assistant message and one tool message per result, between the user's
// text and the reply. Nothing the assistant did is attributed to the
// user, and the reply carries only what the assistant said.
func TestConversationMessages_ActionsReplayAsNativeToolCalls(t *testing.T) {
	messages := LayoutConversation(ai.Prompt{History: []ai.HistoryTurn{{
		User:      "fix it",
		Actions:   []ai.HistoryAction{{Tool: "bash", Args: `command="go test"`, Summary: "Failed: TestX", Failed: true}, {Tool: "writeFile", Args: `content="x" path="a.go"`, Summary: "Wrote a.go"}},
		Assistant: "fixed",
	}}, Text: "thanks"}).Messages()

	require.Len(t, messages, 6)
	assert.Equal(t, ChatMessage{Role: RoleUser, Text: "fix it"}, messages[0])

	calls := messages[1]
	assert.Equal(t, RoleAssistant, calls.Role)
	assert.Empty(t, calls.Text, "the call message carries no prose the model could imitate")
	require.Len(t, calls.ToolCalls, 2)
	assert.Equal(t, ChatToolCall{ID: "hist_0_0", Type: "function", Function: ChatToolCallFunction{Name: "bash", Arguments: json.RawMessage(`{"command":"go test"}`)}}, calls.ToolCalls[0])
	assert.Equal(t, ChatToolCall{ID: "hist_0_1", Type: "function", Function: ChatToolCallFunction{Name: "writeFile", Arguments: json.RawMessage(`{"content":"x","path":"a.go"}`)}}, calls.ToolCalls[1])

	assert.Equal(t, ChatMessage{Role: RoleTool, ToolCallID: "hist_0_0", ToolName: "bash", Text: "Failed: TestX", IsError: true}, messages[2])
	assert.Equal(t, ChatMessage{Role: RoleTool, ToolCallID: "hist_0_1", ToolName: "writeFile", Text: "Wrote a.go"}, messages[3])
	assert.Equal(t, ChatMessage{Role: RoleAssistant, Text: "fixed"}, messages[4])
	assert.Equal(t, ChatMessage{Role: RoleUser, Text: "thanks"}, messages[5])
}

// Call ids are positional (turn, action) so a replayed prefix is
// byte-identical from one request to the next and provider caches match.
func TestConversationMessages_CallIDsFollowTurnPosition(t *testing.T) {
	messages := LayoutConversation(ai.Prompt{History: []ai.HistoryTurn{
		{User: "q1", Actions: []ai.HistoryAction{{Tool: "a"}}, Assistant: "a1"},
		{User: "q2", Actions: []ai.HistoryAction{{Tool: "b"}, {Tool: "c"}}, Assistant: "a2"},
	}, Text: "q3"}).Messages()

	var ids []string
	for _, m := range messages {
		if m.Role == RoleTool {
			ids = append(ids, m.ToolCallID)
		}
	}
	assert.Equal(t, []string{"hist_0_0", "hist_1_0", "hist_1_1"}, ids)
}

// A result without a summary still answers its call: every call needs a
// result or providers reject the request.
func TestConversationMessages_EmptySummaryStillAnswersTheCall(t *testing.T) {
	messages := LayoutConversation(ai.Prompt{History: []ai.HistoryTurn{{User: "q", Actions: []ai.HistoryAction{{Tool: "bash"}}, Assistant: "a"}}, Text: "n"}).Messages()

	require.Len(t, messages, 5)
	assert.Equal(t, ChatMessage{Role: RoleTool, ToolCallID: "hist_0_0", ToolName: "bash", Text: "(no output recorded)"}, messages[2])
}

// The digest cap replaces dropped runs with an omission marker. It is not
// a tool call, so it folds into the neighbouring result as a note.
func TestConversationMessages_OmissionMarkersFoldIntoAdjacentResults(t *testing.T) {
	messages := LayoutConversation(ai.Prompt{History: []ai.HistoryTurn{{User: "q", Actions: []ai.HistoryAction{
		{Tool: ai.OmittedActions, Summary: "3 more actions omitted"},
		{Tool: "readFile", Args: `path="a"`, Summary: "read a"},
		{Tool: ai.OmittedActions, Summary: "7 more actions omitted"},
		{Tool: "bash", Summary: "ok"},
		{Tool: ai.OmittedActions, Summary: "2 more actions omitted"},
	}, Assistant: "a"}}, Text: "n"}).Messages()

	require.Len(t, messages, 6)
	require.Len(t, messages[1].ToolCalls, 2, "markers are not calls")
	assert.Equal(t, "[3 more actions omitted before this]\n\nread a", messages[2].Text)
	assert.Equal(t, "[7 more actions omitted before this]\n\nok\n\n[2 more actions omitted after this]", messages[3].Text)
}

// A turn whose digest is only markers has nothing to call. The note is
// the assistant's doing, so it opens the reply, never the user's text.
func TestConversationMessages_MarkersAloneOpenTheReply(t *testing.T) {
	messages := LayoutConversation(ai.Prompt{History: []ai.HistoryTurn{{User: "q", Actions: []ai.HistoryAction{{Tool: ai.OmittedActions, Summary: "9 more actions omitted"}}, Assistant: "a"}}, Text: "n"}).Messages()

	require.Len(t, messages, 3)
	assert.Equal(t, ChatMessage{Role: RoleUser, Text: "q"}, messages[0])
	assert.Equal(t, ChatMessage{Role: RoleAssistant, Text: "[9 more actions omitted]\n\na"}, messages[1])
}

// A seeded turn can hold actions without a user side (the host saw the
// agent act first). Providers require a user message before a tool-call
// message, so a placeholder fills the slot and the calls stay the
// assistant's.
func TestConversationMessages_ActionsWithoutUserTextGetAPlaceholderUserMessage(t *testing.T) {
	messages := LayoutConversation(ai.Prompt{History: []ai.HistoryTurn{{Actions: []ai.HistoryAction{{Tool: "bash", Summary: "ok"}}, Assistant: "done"}}, Text: "n"}).Messages()

	require.Len(t, messages, 5)
	assert.Equal(t, ChatMessage{Role: RoleUser, Text: ContinuedTurn}, messages[0])
	assert.Equal(t, RoleAssistant, messages[1].Role)
	require.Len(t, messages[1].ToolCalls, 1)
	assert.Equal(t, RoleTool, messages[2].Role)
	assert.Equal(t, ChatMessage{Role: RoleAssistant, Text: "done"}, messages[3])
}

// A turn may end on its results (no reply text); the next turn's user
// message then follows the last result directly, and the pairing of
// calls to results is unaffected.
func TestConversationMessages_TurnEndingOnResultsThenNextTurn(t *testing.T) {
	messages := LayoutConversation(ai.Prompt{History: []ai.HistoryTurn{
		{User: "q1", Actions: []ai.HistoryAction{{Tool: "bash", Summary: "ok"}}},
		{User: "q2", Actions: []ai.HistoryAction{{Tool: "readFile", Summary: "read"}}, Assistant: "a2"},
	}, Text: "q3"}).Messages()

	require.Len(t, messages, 8)
	assert.Equal(t, RoleTool, messages[2].Role)
	assert.Equal(t, ChatMessage{Role: RoleUser, Text: "q2"}, messages[3])
	assert.Equal(t, "hist_1_0", messages[4].ToolCalls[0].ID)
	assert.Equal(t, "hist_1_0", messages[5].ToolCallID)
	assert.Equal(t, ChatMessage{Role: RoleAssistant, Text: "a2"}, messages[6])
}

func TestConversationMessages_SystemHistoryThenTail(t *testing.T) {
	c := LayoutConversation(ai.Prompt{
		Instruction: "be kind",
		Context:     ai.TurnContext{Project: "# AGENTS.md", Host: "[Memory]"},
		History:     []ai.HistoryTurn{{User: "q1", Assistant: "a1"}, {Assistant: "seeded"}, {User: "q2"}},
		Text:        "q3",
		Images:      []*ai.Image{{Type: "image/png", Data: []byte{1}}},
	})

	messages := c.Messages()

	require.Len(t, messages, 6)
	assert.Equal(t, ChatMessage{Role: RoleSystem, Text: "be kind\n\n# AGENTS.md"}, messages[0])
	assert.Equal(t, ChatMessage{Role: RoleUser, Text: "q1"}, messages[1])
	assert.Equal(t, ChatMessage{Role: RoleAssistant, Text: "a1"}, messages[2])
	assert.Equal(t, ChatMessage{Role: RoleAssistant, Text: "seeded"}, messages[3])
	assert.Equal(t, ChatMessage{Role: RoleUser, Text: "q2"}, messages[4])
	assert.Equal(t, ChatMessage{Role: RoleUser, Text: "[Memory]\n\nq3", Images: c.Images}, messages[5])
}

func TestConversationMessages_NoSystemWithoutInstruction(t *testing.T) {
	messages := LayoutConversation(ai.Prompt{Text: "hi"}).Messages()

	require.Len(t, messages, 1)
	assert.Equal(t, RoleUser, messages[0].Role)
}

func TestLayoutConversation_CarriesTheRawMessage(t *testing.T) {
	c := LayoutConversation(ai.Prompt{Message: "hi there", Text: "# CURRENT MESSAGE\nUser: hi there"})

	assert.Equal(t, "hi there", c.Message)
	assert.Equal(t, "# CURRENT MESSAGE\nUser: hi there", c.Text)
}
