package ctx

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Chat history is correctness state: the core records turns
// synchronously via AddTurn. The provider must not also build history
// from bus events, or turns would be double-recorded and history would
// depend on asynchronous delivery.
func TestChatProviderDoesNotRecordFromBusEvents(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewChatCtxManager(bus)

	event := events.ChatResponseEvent{
		Message:  "a question",
		Response: "an answer",
	}
	bus.PublishSync(event.Topic(), event)

	part, err := provider.GetPart(context.Background())
	require.NoError(t, err)
	assert.Empty(t, part.Content, "history must come from AddTurn, not from bus events")
}

func TestChatProviderAddTurn(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewChatCtxManager(bus)

	provider.AddTurn("first question", "complete answer")
	provider.AddTurn("", "assistant-only note") // ephemeral input
	provider.AddTurn("user-only note", "")      // ephemeral output

	part, err := provider.GetPart(context.Background())
	require.NoError(t, err)

	assert.Contains(t, part.Content, "first question")
	assert.Contains(t, part.Content, "complete answer")
	assert.Contains(t, part.Content, "assistant-only note")
	assert.Contains(t, part.Content, "user-only note")
}

func TestChatProviderAddTurnSkipsEmptyTurns(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewChatCtxManager(bus)

	provider.AddTurn("", "")

	part, err := provider.GetPart(context.Background())
	require.NoError(t, err)
	assert.Empty(t, part.Content)
}

// A turn's activities render as an Actions block between the user and
// assistant sides, so the next turn sees what the previous one did.
func TestChatCtxManager_RendersTurnActivities(t *testing.T) {
	manager := NewChatCtxManager(events.NewEventBus())

	manager.AddTurn("fix the tests", "Fixed and verified.",
		events.ToolActivity{Tool: "bash", Args: `command="go test ./..."`, Success: false, Summary: "Failed: TestResponsesUsage"},
		events.ToolActivity{Tool: "edit", Args: `file="turn.go"`, Success: true, Summary: "Executed"},
	)

	part, err := manager.GetPart(context.Background())
	require.NoError(t, err)
	assert.Contains(t, part.Content,
		"User: fix the tests\n"+
			"Actions:\n"+
			"- bash command=\"go test ./...\" → Failed: TestResponsesUsage\n"+
			"- edit file=\"turn.go\" → Executed\n"+
			"Assistant: Fixed and verified.")
}

// Activities are part of the message, so the budget formatter must
// include them: what the model sees is exactly what was counted.
func TestFormatMessageForContextIncludesActivities(t *testing.T) {
	formatted := formatMessageForContext(Message{
		User:       "fix",
		Activities: []events.ToolActivity{{Tool: "bash", Summary: "Executed"}},
		Assistant:  "done",
	})

	assert.Contains(t, formatted, "Actions:\n- bash → Executed")
}

// Seeded history carries activities through unchanged, so a restored
// turn renders identically to one recorded live.
func TestChatCtxManager_SeedHistoryPreservesActivities(t *testing.T) {
	manager := NewChatCtxManager(events.NewEventBus())

	manager.SeedHistory([]Message{{
		User:       "earlier request",
		Activities: []events.ToolActivity{{Tool: "writeFile", Args: `path="a.go"`, Success: true, Summary: "Executed"}},
		Assistant:  "earlier answer",
	}})

	part, err := manager.GetPart(context.Background())
	require.NoError(t, err)
	assert.Contains(t, part.Content, "Actions:\n- writeFile path=\"a.go\" → Executed")
}
