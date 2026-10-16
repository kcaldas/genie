package controllers

import (
	"sync"
	"testing"
	"time"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	core_events "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestConfigManager creates a ConfigManager for testing
func createTestConfigManager() *helpers.ConfigManager {
	cm, err := helpers.NewConfigManager()
	if err != nil {
		panic("Failed to create test config manager: " + err.Error())
	}
	return cm
}

// recordingDispatcher records every message a controller dispatches to the
// UI. Controllers dispatch from event-bus goroutines, so it is safe for
// concurrent use.
type recordingDispatcher struct {
	mu   sync.Mutex
	msgs []any
}

func (d *recordingDispatcher) Dispatch(msg any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.msgs = append(d.msgs, msg)
}

// mockCommandHandler implements CommandHandler for testing
type mockCommandHandler struct {
	commands       map[string]func([]string) error
	lastCommand    string
	lastArgs       []string
	commandHistory []string
}

func (m *mockCommandHandler) GetAvailableCommands() []string {
	var commands []string
	for cmd := range m.commands {
		commands = append(commands, cmd)
	}
	return commands
}

func TestChatController_HandleInput(t *testing.T) {
	scenarios := []struct {
		name              string
		input             string
		expectChatMessage bool
	}{
		{
			name:              "regular message",
			input:             "hello world",
			expectChatMessage: true,
		},
		{
			name:              "empty input",
			input:             "",
			expectChatMessage: true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			// Setup
			chatState := state.NewChatState(100)
			uiState := state.NewUIState()
			stateAccessor := state.NewStateAccessor(chatState, uiState)

			ui := &recordingDispatcher{}

			// Create test fixture for genie
			fixture := genietest.NewTestFixture(t)
			fixture.StartAndGetSession() // Start genie before use

			eventBus := events.NewCommandEventBus()
			controller := NewChatController(
				ui,
				fixture.Genie,
				stateAccessor,
				createTestConfigManager(),
				eventBus,
			)

			// Execute
			err := controller.handleChatMessage(s.input)

			// Verify
			require.NoError(t, err)

			if s.expectChatMessage && s.input != "" {
				messages := stateAccessor.GetMessages()
				assert.NotEmpty(t, messages, "Expected message to be added")
				if len(messages) > 0 {
					assert.Equal(t, "user", messages[0].Role)
					assert.Equal(t, s.input, messages[0].Content)
				}
			} else if !s.expectChatMessage {
				messages := stateAccessor.GetMessages()
				assert.Empty(t, messages, "Expected no chat message to be added for commands")
			}
		})
	}
}

func TestChatController_ClearConversation(t *testing.T) {
	// Setup
	chatState := state.NewChatState(100)
	uiState := state.NewUIState()
	stateAccessor := state.NewStateAccessor(chatState, uiState)

	// Add some messages
	stateAccessor.AddMessage(types.Message{Role: "user", Content: "test1"})
	stateAccessor.AddMessage(types.Message{Role: "assistant", Content: "test2"})

	ui := &recordingDispatcher{}

	fixture := genietest.NewTestFixture(t)

	eventBus := events.NewCommandEventBus()
	controller := NewChatController(
		ui,
		fixture.Genie,
		stateAccessor,
		createTestConfigManager(),
		eventBus,
	)

	// Verify messages exist
	assert.Equal(t, 2, len(stateAccessor.GetMessages()))

	// Execute
	err := controller.ClearConversation()

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 0, len(stateAccessor.GetMessages()))
}

func TestChatController_GetConversationHistory(t *testing.T) {
	// Setup
	chatState := state.NewChatState(100)
	uiState := state.NewUIState()
	stateAccessor := state.NewStateAccessor(chatState, uiState)

	expectedMessages := []types.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you?"},
	}

	for _, msg := range expectedMessages {
		stateAccessor.AddMessage(msg)
	}

	ui := &recordingDispatcher{}

	fixture := genietest.NewTestFixture(t)

	eventBus := events.NewCommandEventBus()
	controller := NewChatController(
		ui,
		fixture.Genie,
		stateAccessor,
		createTestConfigManager(),
		eventBus,
	)

	// Execute
	history := controller.GetConversationHistory()

	// Verify
	require.Len(t, history, len(expectedMessages))
	for i, msg := range expectedMessages {
		assert.Equal(t, msg.Role, history[i].Role)
		assert.Equal(t, msg.Content, history[i].Content)
	}
}

func TestThinkingDeltasMergeIntoOneMessage(t *testing.T) {
	chatState := state.NewChatState(100)
	stateAccessor := state.NewStateAccessor(chatState, state.NewUIState())
	fixture := genietest.NewTestFixture(t)
	fixture.StartAndGetSession()
	NewChatController(&recordingDispatcher{}, fixture.Genie, stateAccessor, createTestConfigManager(), events.NewCommandEventBus())

	bus := fixture.EventBus
	bus.PublishSync("chat.notification", core_events.NotificationEvent{Message: "first ", ContentType: "thought"})
	bus.PublishSync("chat.notification", core_events.NotificationEvent{Message: "second", ContentType: "thought"})
	bus.PublishSync("chat.notification", core_events.NotificationEvent{Message: "answer", ContentType: "text"})
	bus.PublishSync("chat.notification", core_events.NotificationEvent{Message: "later", ContentType: "thought"})

	msgs := chatState.GetMessages()
	if len(msgs) != 3 || msgs[0].Content != "first second" || msgs[0].ContentType != "thought" || msgs[2].Content != "later" {
		t.Fatalf("messages = %+v", msgs)
	}
}

func TestHiddenTodoWriteStillUpdatesTodos(t *testing.T) {
	chatState := state.NewChatState(100)
	stateAccessor := state.NewStateAccessor(chatState, state.NewUIState())
	fixture := genietest.NewTestFixture(t)
	fixture.StartAndGetSession()
	cm := createTestConfigManager()
	cfg := cm.GetConfig()
	if cfg.ToolConfigs == nil {
		cfg.ToolConfigs = map[string]types.ToolConfig{}
	}
	cfg.ToolConfigs["TodoWrite"] = types.ToolConfig{Hide: true}
	ui := &recordingDispatcher{}
	NewChatController(ui, fixture.Genie, stateAccessor, cm, events.NewCommandEventBus())

	fixture.EventBus.PublishSync("tool.executed", core_events.ToolExecutedEvent{
		ToolName: "TodoWrite", Success: true,
		Result: map[string]any{"success": true, "todos": []any{map[string]any{"id": "1", "content": "x", "status": "pending"}}},
	})
	if chatState.GetMessageCount() != 0 {
		t.Fatal("a hidden tool must not add a card")
	}
	ui.mu.Lock()
	defer ui.mu.Unlock()
	found := false
	for _, m := range ui.msgs {
		if u, ok := m.(uimsg.TodosUpdated); ok && len(u.Items) == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("todos not dispatched for a hidden TodoWrite: %#v", ui.msgs)
	}
}

func newQueueTestController(t *testing.T) (*ChatController, *genietest.TestFixture, *state.ChatState, *events.CommandEventBus, *recordingDispatcher) {
	t.Helper()
	chatState := state.NewChatState(100)
	stateAccessor := state.NewStateAccessor(chatState, state.NewUIState())
	fixture := genietest.NewTestFixture(t)
	fixture.StartAndGetSession()
	bus := events.NewCommandEventBus()
	ui := &recordingDispatcher{}
	c := NewChatController(ui, fixture.Genie, stateAccessor, createTestConfigManager(), bus)
	return c, fixture, chatState, bus, ui
}

func queueCounts(ui *recordingDispatcher) []int {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	var out []int
	for _, m := range ui.msgs {
		if q, ok := m.(uimsg.QueueChanged); ok {
			out = append(out, q.Count)
		}
	}
	return out
}

func TestMessagesTypedWhileRunningAreQueuedThenSent(t *testing.T) {
	c, fixture, chatState, bus, ui := newQueueTestController(t)
	fixture.ExpectMessage("second").RespondWith("answer two")

	c.requestManager.StartRequest() // simulate a request in flight
	bus.Emit("user.input.text", "second")
	bus.WaitForPendingEvents()
	if c.QueuedCount() != 1 {
		t.Fatalf("queued = %d, want 1", c.QueuedCount())
	}
	if last := chatState.GetLastMessage(); last == nil || last.Content != "Queued (1): second" {
		t.Fatalf("queued notice = %+v", last)
	}

	// The running request finishes: the queued message goes out and is answered.
	fixture.EventBus.PublishSync("chat.response", core_events.ChatResponseEvent{RequestID: "fake", Response: "answer one"})
	// The fixture also sees the fake response; wait for the real one.
	var resp *core_events.ChatResponseEvent
	for i := 0; i < 2; i++ {
		resp = fixture.WaitForResponseOrFail(5 * time.Second)
		if resp.Response == "answer two" {
			break
		}
	}
	if resp.Response != "answer two" {
		t.Fatalf("queued message not sent: %+v", resp)
	}
	if c.QueuedCount() != 0 {
		t.Fatal("queue should drain")
	}
	if counts := queueCounts(ui); len(counts) < 2 || counts[0] != 1 || counts[len(counts)-1] != 0 {
		t.Fatalf("queue counts = %v", counts)
	}
}

func TestCancelDropsTheQueueAndSaysSo(t *testing.T) {
	c, _, chatState, bus, _ := newQueueTestController(t)
	c.requestManager.StartRequest()
	bus.Emit("user.input.text", "later on")
	bus.WaitForPendingEvents()
	c.CancelChat()
	if c.QueuedCount() != 0 {
		t.Fatal("cancel must drop queued messages")
	}
	found := false
	for _, m := range chatState.GetMessages() {
		if m.Content == "Dropped queued message: later on" {
			found = true
		}
	}
	if !found {
		t.Fatalf("dropped text must be shown: %+v", chatState.GetMessages())
	}
	c.requestManager.StartRequest()
	bus.Emit("user.input.text", "gone")
	bus.WaitForPendingEvents()
	c.ClearConversation()
	if c.QueuedCount() != 0 {
		t.Fatal(":clear must drop the queue too")
	}
}
