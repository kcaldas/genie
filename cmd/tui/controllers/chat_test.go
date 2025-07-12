package controllers

import (
	"testing"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(message string) {
	// Do nothing in tests
}

// createTestConfigManager creates a ConfigManager for testing
func createTestConfigManager() *helpers.ConfigManager {
	cm, err := helpers.NewConfigManager()
	if err != nil {
		panic("Failed to create test config manager: " + err.Error())
	}
	return cm
}

// mockGuiCommon implements types.IGuiCommon for testing
type mockGuiCommon struct {
	updateCallbacks []func()
}

func (m *mockGuiCommon) GetGui() *gocui.Gui { return nil }
func (m *mockGuiCommon) GetTheme() *types.Theme {
	return &types.Theme{
		Primary: "\033[36m",
	}
}
func (m *mockGuiCommon) SetCurrentComponent(ctx types.Component) {}
func (m *mockGuiCommon) GetCurrentComponent() types.Component    { return nil }
func (m *mockGuiCommon) PostUIUpdate(fn func()) {
	m.updateCallbacks = append(m.updateCallbacks, fn)
	fn() // Execute immediately for testing
}

// mockComponent implements types.Component for testing
type mockComponent struct {
	key          string
	viewName     string
	windowName   string
	keybindings  []*types.KeyBinding
	focusCount   int
	unfocusCount int
}

func (m *mockComponent) GetKey() string                      { return m.key }
func (m *mockComponent) GetView() *gocui.View                { return nil }
func (m *mockComponent) GetViewName() string                 { return m.viewName }
func (m *mockComponent) GetWindowName() string               { return m.windowName }
func (m *mockComponent) HandleFocus() error                  { m.focusCount++; return nil }
func (m *mockComponent) HandleFocusLost() error              { m.unfocusCount++; return nil }
func (m *mockComponent) GetKeybindings() []*types.KeyBinding { return m.keybindings }
func (m *mockComponent) Render() error                       { return nil }
func (m *mockComponent) HasControlledBounds() bool           { return true }
func (m *mockComponent) IsTransient() bool                   { return false }
func (m *mockComponent) GetWindowProperties() types.WindowProperties {
	return types.WindowProperties{
		Focusable: true, Editable: false, Wrap: true,
		Autoscroll: false, Highlight: true, Frame: true,
	}
}
func (m *mockComponent) GetTitle() string { return "Mock" }

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

			guiCommon := &mockGuiCommon{}
			context := &mockComponent{key: "test", viewName: "test"}

			// Create test fixture for genie
			fixture := genie.NewTestFixture(t)
			fixture.StartAndGetSession() // Start genie before use

			eventBus := events.NewCommandEventBus()
			controller := NewChatController(
				context,
				guiCommon,
				fixture.Genie,
				stateAccessor,
				createTestConfigManager(),
				eventBus,
				&mockLogger{},
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

	guiCommon := &mockGuiCommon{}
	context := &mockComponent{key: "test", viewName: "test"}

	fixture := genie.NewTestFixture(t)

	eventBus := events.NewCommandEventBus()
	controller := NewChatController(
		context,
		guiCommon,
		fixture.Genie,
		stateAccessor,
		createTestConfigManager(),
		eventBus,
		&mockLogger{},
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

	guiCommon := &mockGuiCommon{}
	context := &mockComponent{key: "test", viewName: "test"}

	fixture := genie.NewTestFixture(t)

	eventBus := events.NewCommandEventBus()
	controller := NewChatController(
		context,
		guiCommon,
		fixture.Genie,
		stateAccessor,
		createTestConfigManager(),
		eventBus,
		&mockLogger{},
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
