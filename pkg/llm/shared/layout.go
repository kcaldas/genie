package shared

import (
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

// Conversation is the provider-neutral wire layout of one model call.
// Every client maps it onto its own message types without deciding order
// or placement itself.
//
//	System   stable, agent-wide: the rendered instruction and the
//	         workspace's project context
//	Turns    the conversation so far, one message pair per turn — an
//	         append-only prefix
//	Context  volatile per-turn context: files, active skill, host context,
//	         tasks
//	Text     the current message
//
// Context and Text form the final user message together, so the only
// content that differs between one turn and the next sits at the very
// end of the request. That is what lets provider prompt caches match
// everything in front of it.
type Conversation struct {
	System  string
	Turns   []ai.HistoryTurn
	Context string
	Text    string
	Images  []*ai.Image
}

// LayoutConversation arranges a rendered prompt into wire order.
func LayoutConversation(p ai.Prompt) Conversation {
	return Conversation{
		System:  joinBlocks(p.Instruction, p.Context.Project),
		Turns:   p.History,
		Context: joinBlocks(p.Context.Files, p.Context.Skill, p.Context.Host, p.Context.Tasks),
		Text:    strings.TrimSpace(p.Text),
		Images:  p.Images,
	}
}

// TailText is the text of the final user message: the volatile context,
// then the current message.
func (c Conversation) TailText() string {
	return joinBlocks(c.Context, c.Text)
}

// FormatAssistantTurn renders what the assistant did in a past turn as one
// message: the tool actions it took, then its answer.
func FormatAssistantTurn(turn ai.HistoryTurn) string {
	var actions string
	if len(turn.Actions) > 0 {
		lines := make([]string, 0, len(turn.Actions)+1)
		lines = append(lines, "Assistant Actions:")
		for _, action := range turn.Actions {
			line := "- " + action.Tool
			if action.Args != "" {
				line += " " + action.Args
			}
			if action.Summary != "" {
				line += " → " + action.Summary
			}
			lines = append(lines, line)
		}
		actions = strings.Join(lines, "\n")
	}
	return joinBlocks(actions, turn.Assistant)
}

// joinBlocks joins non-empty, trimmed blocks with a blank line.
func joinBlocks(blocks ...string) string {
	kept := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block = strings.TrimSpace(block); block != "" {
			kept = append(kept, block)
		}
	}
	return strings.Join(kept, "\n\n")
}
