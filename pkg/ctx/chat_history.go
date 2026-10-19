package ctx

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
)

// Message represents a user/assistant conversation pair, with the tool
// activities that produced the assistant side.
type Message struct {
	User       string
	Activities []events.ToolActivity
	Assistant  string
}

// ChatHistory holds a session's conversation turns: what the model sees
// as its past, kept under a token budget. (Unrelated to cmd/history's
// input-line history.) It is not a context part.
// Providers lay the turns out as native messages (user text, tool calls,
// tool results, reply) from the structured Message values; the text
// rendering in FormatHistory exists for budgeting, recording and display.
type ChatHistory interface {
	SetTokenBudget(tokens int)
	SetBudgetStrategy(strategy CollectionBudgetStrategy[Message])
	// History returns the turns the model will see this read, oldest
	// first, after any budget prune. Callers get a copy.
	History(ctx context.Context) []Message
	// AddTurn records one completed exchange, optionally with the tool
	// activities that produced it. Empty user or assistant sides are
	// allowed (ephemeral modes); a fully empty turn is ignored.
	AddTurn(user, assistant string, activities ...events.ToolActivity)
	// SeedHistory replaces the history with turns restored by a host.
	SeedHistory(history []Message)
	// Clear forgets every turn.
	Clear()
}

// InMemoryChatHistory implements ChatHistory with in-memory storage.
type InMemoryChatHistory struct {
	mu             sync.RWMutex
	messages       []Message
	budgetStrategy CollectionBudgetStrategy[Message]
	tokenBudget    int

	publisher events.Publisher
}

// NewChatHistory creates an empty in-memory chat history.
//
// History is recorded synchronously by the core via AddTurn after each
// successful turn — never from bus events, whose delivery order and
// timing must not influence what the model remembers.
//
// The event bus is used for outbound observability only: prune outcomes
// are published as ContextPrunedEvent for observers such as the session
// recorder.
func NewChatHistory(eventBus events.EventBus) ChatHistory {
	return &InMemoryChatHistory{
		messages:  make([]Message, 0),
		publisher: eventBus,
	}
}

// AddTurn records one completed exchange in conversation history.
func (h *InMemoryChatHistory) AddTurn(user, assistant string, activities ...events.ToolActivity) {
	if user == "" && assistant == "" {
		return
	}

	h.mu.Lock()
	h.messages = append(h.messages, Message{
		User:       user,
		Activities: activities,
		Assistant:  assistant,
	})
	h.mu.Unlock()
}

// SetBudgetStrategy sets the collection budget strategy for the history.
func (h *InMemoryChatHistory) SetBudgetStrategy(strategy CollectionBudgetStrategy[Message]) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.budgetStrategy = strategy
}

// SetTokenBudget sets the token budget the history is pruned to.
func (h *InMemoryChatHistory) SetTokenBudget(tokens int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tokenBudget = tokens
}

// FormatMessage renders one turn as text: the user's words, the tool
// activities as "Assistant Actions", then the reply. It is the budget's
// cost function and the recorder's and context viewer's view of a turn.
// The model never sees it: providers replay activities as native tool
// calls from the structured history.
func FormatMessage(msg Message) string {
	var parts []string
	if msg.User != "" {
		parts = append(parts, "User: "+msg.User)
	}
	if len(msg.Activities) > 0 {
		lines := make([]string, 0, len(msg.Activities)+1)
		// "Assistant Actions" names the actor explicitly, matching the
		// User:/Assistant: speaker labels around it.
		lines = append(lines, "Assistant Actions:")
		for _, activity := range msg.Activities {
			line := "- " + activity.Tool
			if activity.Args != "" {
				line += " " + activity.Args
			}
			if activity.Summary != "" {
				line += " → " + activity.Summary
			}
			lines = append(lines, line)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	if msg.Assistant != "" {
		parts = append(parts, "Assistant: "+msg.Assistant)
	}
	return strings.Join(parts, "\n")
}

// FormatHistory renders turns with FormatMessage, one per line group,
// skipping turns that render empty.
func FormatHistory(messages []Message) string {
	var parts []string
	for _, msg := range messages {
		if formatted := FormatMessage(msg); formatted != "" {
			parts = append(parts, formatted)
		}
	}
	return strings.Join(parts, "\n")
}

// pruneLowWater is the fraction of the token budget the history is cut
// down to when it overflows. Cutting below the budget, not just to it,
// is what keeps the prune from recurring on every turn: a history that
// overflows by one message would otherwise drop its oldest message on
// every read, and each drop shifts the whole prefix the provider's
// prompt cache had matched.
const pruneLowWater = 0.6

// History returns the turns the model will see this read, oldest first,
// after any budget prune. The prune is in place and permanent: turns that
// no longer fit are gone, so the window start stays fixed until the
// history overflows again. Callers get a copy.
func (h *InMemoryChatHistory) History(ctx context.Context) []Message {
	if pruneEvent := h.pruneToBudget(ctx); pruneEvent != nil {
		h.publishPrune(*pruneEvent)
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	history := make([]Message, len(h.messages))
	copy(history, h.messages)
	return history
}

// pruneToBudget drops the oldest turns once the history overflows the
// token budget, keeping only what fits under the low-water mark. It
// reports the prune, or nil when nothing was dropped.
func (h *InMemoryChatHistory) pruneToBudget(ctx context.Context) *events.ContextPrunedEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.budgetStrategy == nil || h.tokenBudget <= 0 || historyTokens(h.messages) <= h.tokenBudget {
		return nil
	}
	lowWater := int(float64(h.tokenBudget) * pruneLowWater)
	kept, keptTokens := h.budgetStrategy.ApplyToCollection(h.messages, lowWater, FormatMessage)
	if len(kept) == 0 {
		// No single turn fits under the low-water mark; keep what fits the
		// full budget rather than emptying the conversation.
		kept, keptTokens = h.budgetStrategy.ApplyToCollection(h.messages, h.tokenBudget, FormatMessage)
	}
	dropped := len(h.messages) - len(kept)
	if dropped <= 0 {
		return nil
	}
	slog.InfoContext(ctx, "chat history pruned",
		"strategy", h.budgetStrategy.Name(),
		"total", len(h.messages),
		"kept", len(kept),
		"dropped", dropped,
		"kept_tokens", keptTokens,
		"budget_tokens", h.tokenBudget,
	)
	event := &events.ContextPrunedEvent{
		Strategy:     h.budgetStrategy.Name(),
		Total:        len(h.messages),
		Kept:         len(kept),
		Dropped:      dropped,
		KeptTokens:   keptTokens,
		BudgetTokens: h.tokenBudget,
	}
	h.messages = kept
	return event
}

func historyTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(FormatMessage(msg))
	}
	return total
}

// publishPrune announces a prune. Every prune is a real change to the
// history now that pruning is in place, so none is deduplicated.
func (h *InMemoryChatHistory) publishPrune(event events.ContextPrunedEvent) {
	if h.publisher == nil {
		return
	}
	h.publisher.PublishSync(event.Topic(), event)
}

// Clear forgets every turn.
func (h *InMemoryChatHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = make([]Message, 0)
}

// SeedHistory replaces the current chat history with the provided messages.
func (h *InMemoryChatHistory) SeedHistory(history []Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(history) == 0 {
		h.messages = make([]Message, 0)
		return
	}

	h.messages = make([]Message, 0, len(history))
	for _, msg := range history {
		if msg.User == "" && msg.Assistant == "" {
			continue
		}
		h.messages = append(h.messages, Message{
			User:       msg.User,
			Activities: msg.Activities,
			Assistant:  msg.Assistant,
		})
	}
}
