package shared

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
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
