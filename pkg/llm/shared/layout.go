package shared

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

// Conversation is the provider-neutral wire layout of one model call.
// Every client maps it onto its own message types without deciding order
// or placement itself.
//
//	System   stable, agent-wide: the rendered instruction and the
//	         workspace's project context
//	Turns    the conversation so far, each turn as its messages (user
//	         text, tool calls and results, reply) — an append-only prefix
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
	RoleTool      = "tool"
)

// ChatMessage is one message of the OpenAI-style chat shape: a role and
// its text, plus images on the final user message only. An assistant
// message may carry ToolCalls instead of text; a RoleTool message is the
// result of one of them, correlated by ToolCallID.
type ChatMessage struct {
	Role   string
	Text   string
	Images []*ai.Image

	ToolCalls  []ChatToolCall // RoleAssistant: calls made before the reply
	ToolCallID string         // RoleTool: the call this result answers
	ToolName   string         // RoleTool: the tool that ran
	IsError    bool           // RoleTool: the tool failed
}

// Messages renders the conversation in the OpenAI-style chat shape: the
// system message, each past turn as its messages, then the current turn
// as the final user message. Clients that speak that shape map each
// message onto their wire type and add nothing of their own.
//
// A past turn is the user's text, then — when the assistant used tools —
// an assistant message carrying the calls and one tool message per
// result, then the assistant's reply. The calls replay as what they were,
// so the model never reads a digest of its own actions as something the
// user said, nor as prose in its own voice to imitate.
func (c Conversation) Messages() []ChatMessage {
	var messages []ChatMessage
	if c.System != "" {
		messages = append(messages, ChatMessage{Role: RoleSystem, Text: c.System})
	}
	for i, turn := range c.Turns {
		messages = append(messages, turnMessages(i, turn)...)
	}
	return append(messages, ChatMessage{Role: RoleUser, Text: c.TailText(), Images: c.Images})
}

// TailText is the text of the final user message: the volatile context,
// then the current message.
func (c Conversation) TailText() string {
	return joinBlocks(c.Context, c.Text)
}

// ContinuedTurn stands in for the user side of a turn that has calls but
// no user text (a host seeded the agent acting first). Providers require
// a user message before a tool-call message, so the slot cannot be empty.
const ContinuedTurn = "(continued)"

func turnMessages(index int, turn ai.HistoryTurn) []ChatMessage {
	var messages []ChatMessage
	calls, results, note := historyToolMessages(index, turn.Actions)
	user := strings.TrimSpace(turn.User)
	if user == "" && len(calls) > 0 {
		user = ContinuedTurn
	}
	if user != "" {
		messages = append(messages, ChatMessage{Role: RoleUser, Text: user})
	}
	if len(calls) > 0 {
		messages = append(messages, ChatMessage{Role: RoleAssistant, ToolCalls: calls})
		messages = append(messages, results...)
	}
	// A digest of markers alone is still the assistant's doing: the note
	// opens its reply rather than closing the user's text.
	if assistant := joinBlocks(note, turn.Assistant); assistant != "" {
		messages = append(messages, ChatMessage{Role: RoleAssistant, Text: assistant})
	}
	return messages
}

// historyToolMessages converts a turn's digest into its calls and
// results. Ids are positional so a replayed prefix is byte-identical
// across requests. Omission markers are not calls: each folds into the
// result that follows it (trailing ones into the last result), or, when
// the digest holds nothing else, comes back as a note for the reply.
func historyToolMessages(turn int, actions []ai.HistoryAction) (calls []ChatToolCall, results []ChatMessage, note string) {
	var pending []string // markers waiting for the result that follows them
	for _, action := range actions {
		if action.Tool == ai.OmittedActions {
			pending = append(pending, action.Summary)
			continue
		}
		id := fmt.Sprintf("hist_%d_%d", turn, len(calls))
		calls = append(calls, ChatToolCall{
			ID:       id,
			Type:     "function",
			Function: ChatToolCallFunction{Name: action.Tool, Arguments: ActionArguments(action.Args)},
		})
		text := strings.TrimSpace(action.Summary)
		if text == "" {
			text = "(no output recorded)"
		}
		for i := len(pending) - 1; i >= 0; i-- {
			text = joinBlocks("["+pending[i]+" before this]", text)
		}
		pending = nil
		results = append(results, ChatMessage{Role: RoleTool, ToolCallID: id, ToolName: action.Tool, Text: text, IsError: action.Failed})
	}
	switch {
	case len(calls) == 0 && len(pending) > 0:
		notes := make([]string, len(pending))
		for i, p := range pending {
			notes[i] = "[" + p + "]"
		}
		note = strings.Join(notes, "\n")
	case len(pending) > 0:
		last := &results[len(results)-1]
		for _, p := range pending {
			last.Text = joinBlocks(last.Text, "["+p+" after this]")
		}
	}
	return calls, results, note
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
