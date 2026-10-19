package ctx

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

// Chat history is correctness state: the core records turns
// synchronously via AddTurn. The history must not also build itself
// from bus events, or turns would be double-recorded and history would
// depend on asynchronous delivery.
func TestChatHistoryDoesNotRecordFromBusEvents(t *testing.T) {
	bus := events.NewEventBus()
	chat := NewChatHistory(bus)

	event := events.ChatResponseEvent{
		Message:  "a question",
		Response: "an answer",
	}
	bus.PublishSync(event.Topic(), event)

	part := FormatHistory(chat.History(context.Background()))
	assert.Empty(t, part, "history must come from AddTurn, not from bus events")
}

func TestChatHistoryAddTurn(t *testing.T) {
	bus := events.NewEventBus()
	chat := NewChatHistory(bus)

	chat.AddTurn("first question", "complete answer")
	chat.AddTurn("", "assistant-only note") // ephemeral input
	chat.AddTurn("user-only note", "")      // ephemeral output

	part := FormatHistory(chat.History(context.Background()))

	assert.Contains(t, part, "first question")
	assert.Contains(t, part, "complete answer")
	assert.Contains(t, part, "assistant-only note")
	assert.Contains(t, part, "user-only note")
}

func TestChatHistoryAddTurnSkipsEmptyTurns(t *testing.T) {
	bus := events.NewEventBus()
	chat := NewChatHistory(bus)

	chat.AddTurn("", "")

	part := FormatHistory(chat.History(context.Background()))
	assert.Empty(t, part)
}

// A turn's activities render as an Actions block between the user and
// assistant sides in the text view (recorder, context viewer, budget).
func TestChatHistory_RendersTurnActivities(t *testing.T) {
	manager := NewChatHistory(events.NewEventBus())

	manager.AddTurn("fix the tests", "Fixed and verified.",
		events.ToolActivity{Tool: "bash", Args: `command="go test ./..."`, Success: false, Summary: "Failed: TestResponsesUsage"},
		events.ToolActivity{Tool: "edit", Args: `file="turn.go"`, Success: true, Summary: "Executed"},
	)

	part := FormatHistory(manager.History(context.Background()))
	assert.Contains(t, part,
		"User: fix the tests\n"+
			"Assistant Actions:\n"+
			"- bash command=\"go test ./...\" → Failed: TestResponsesUsage\n"+
			"- edit file=\"turn.go\" → Executed\n"+
			"Assistant: Fixed and verified.")
}

// Activities are part of the message, so the budget formatter must
// include them: the replayed tool calls cost tokens too.
func TestFormatMessageIncludesActivities(t *testing.T) {
	formatted := FormatMessage(Message{
		User:       "fix",
		Activities: []events.ToolActivity{{Tool: "bash", Summary: "Executed"}},
		Assistant:  "done",
	})

	assert.Contains(t, formatted, "Assistant Actions:\n- bash → Executed")
}

// Seeded history carries activities through unchanged, so a restored
// turn renders identically to one recorded live.
func TestChatHistory_SeedHistoryPreservesActivities(t *testing.T) {
	manager := NewChatHistory(events.NewEventBus())

	manager.SeedHistory([]Message{{
		User:       "earlier request",
		Activities: []events.ToolActivity{{Tool: "writeFile", Args: `path="a.go"`, Success: true, Summary: "Executed"}},
		Assistant:  "earlier answer",
	}})

	part := FormatHistory(manager.History(context.Background()))
	assert.Contains(t, part, "Assistant Actions:\n- writeFile path=\"a.go\" → Executed")
}
