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
//	Message  the current message as the host sent it (see ai.Prompt.Message)
//	Text     the current message as the persona template rendered it
//
// Context and Text form the final user message together, so the only
// content that differs between one turn and the next sits at the very
// end of the request. That is what lets provider prompt caches match
// everything in front of it.
type Conversation struct {
	System  string
	Turns   []ai.HistoryTurn
	Context string
	Message string
	Text    string
	Images  []*ai.Image
}

// LayoutConversation arranges a rendered prompt into wire order.
func LayoutConversation(p ai.Prompt) Conversation {
	return Conversation{
		System:  joinBlocks(p.Instruction, p.Context.Project),
		Turns:   p.History,
		Context: joinBlocks(p.Context.Files, p.Context.Skill, p.Context.Host, p.Context.Tasks),
		Message: strings.TrimSpace(p.Message),
		Text:    strings.TrimSpace(p.Text),
		Images:  p.Images,
	}
}

// Chat roles shared by the OpenAI-compatible wire formats.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// ChatMessage is one message of the OpenAI-style chat shape: a role and
// its text, plus images on the final user message only.
type ChatMessage struct {
	Role   string
	Text   string
	Images []*ai.Image
}

// Messages renders the conversation in the OpenAI-style chat shape: the
// system message, one message per side of each past turn, then the
// current turn as the final user message. Clients that speak that shape
// map each message onto their wire type and add nothing of their own.
func (c Conversation) Messages() []ChatMessage {
	messages := make([]ChatMessage, 0, 2*len(c.Turns)+2)
	if c.System != "" {
		messages = append(messages, ChatMessage{Role: RoleSystem, Text: c.System})
	}
	for _, turn := range c.Turns {
		if user := strings.TrimSpace(turn.User); user != "" {
			messages = append(messages, ChatMessage{Role: RoleUser, Text: user})
		}
		if assistant := FormatAssistantTurn(turn); assistant != "" {
			messages = append(messages, ChatMessage{Role: RoleAssistant, Text: assistant})
		}
	}
	return append(messages, ChatMessage{Role: RoleUser, Text: c.TailText(), Images: c.Images})
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
