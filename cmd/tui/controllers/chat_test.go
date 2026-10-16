package controllers

import (
	"sync"
	"testing"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
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
