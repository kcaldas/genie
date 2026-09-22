package shared

import (
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

func TestFormatAssistantTurn_ActionsThenAnswer(t *testing.T) {
	turn := ai.HistoryTurn{
		Actions:   []ai.HistoryAction{{Tool: "readFile", Args: "a.md", Summary: "12 lines"}, {Tool: "bash"}},
		Assistant: "done",
	}

	assert.Equal(t, "Assistant Actions:\n- readFile a.md → 12 lines\n- bash\n\ndone", FormatAssistantTurn(turn))
}

func TestFormatAssistantTurn_AnswerOnly(t *testing.T) {
	assert.Equal(t, "done", FormatAssistantTurn(ai.HistoryTurn{Assistant: "done"}))
}

func TestFormatAssistantTurn_ActionsOnly(t *testing.T) {
	turn := ai.HistoryTurn{Actions: []ai.HistoryAction{{Tool: "bash", Args: "ls"}}}

	assert.Equal(t, "Assistant Actions:\n- bash ls", FormatAssistantTurn(turn))
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
