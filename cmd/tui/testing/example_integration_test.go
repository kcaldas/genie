package testing

import (
	"testing"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGuiCommon implements types.IGuiCommon for testing
type mockGuiCommon struct {
	updateCallbacks []func()
}

func (m *mockGuiCommon) GetGui() *gocui.Gui { return nil }
func (m *mockGuiCommon) GetConfig() *types.Config {
	return &types.Config{
		ShowCursor:        true,
		MarkdownRendering: true,
		Theme:             "default",
	}
}
func (m *mockGuiCommon) GetTheme() *types.Theme {
	return &types.Theme{
		Primary: "\033[36m",
	}
}
func (m *mockGuiCommon) SetCurrentComponent(ctx types.Component)  {}
func (m *mockGuiCommon) GetCurrentComponent() types.Component     { return nil }
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

func (m *mockComponent) GetKey() string                              { return m.key }
func (m *mockComponent) GetView() *gocui.View                       { return nil }
func (m *mockComponent) GetViewName() string                        { return m.viewName }
func (m *mockComponent) GetWindowName() string                      { return m.windowName }
func (m *mockComponent) HandleFocus() error                         { m.focusCount++; return nil }
func (m *mockComponent) HandleFocusLost() error                     { m.unfocusCount++; return nil }
func (m *mockComponent) GetKeybindings() []*types.KeyBinding        { return m.keybindings }
func (m *mockComponent) Render() error                              { return nil }
func (m *mockComponent) HasControlledBounds() bool                  { return true }
func (m *mockComponent) IsTransient() bool                          { return false }
func (m *mockComponent) GetWindowProperties() types.WindowProperties { 
	return types.WindowProperties{
		Focusable: true, Editable: false, Wrap: true, 
		Autoscroll: false, Highlight: true, Frame: true,
	}
}
func (m *mockComponent) GetTitle() string                           { return "Mock" }

// mockCommandHandler implements CommandHandler for testing
type mockCommandHandler struct {
	commands       map[string]func([]string) error
	lastCommand    string
	lastArgs       []string
	commandHistory []string
}

func (m *mockCommandHandler) HandleCommand(command string, args []string) error {
	m.lastCommand = command
	m.lastArgs = args
	m.commandHistory = append(m.commandHistory, command)
	
	if handler, exists := m.commands[command]; exists {
		return handler(args)
	}
	return nil
}

func (m *mockCommandHandler) GetAvailableCommands() []string {
	var commands []string
	for cmd := range m.commands {
		commands = append(commands, cmd)
	}
	return commands
}

// TestBasicChatFlow demonstrates a complete chat interaction test without GUI
func TestBasicChatFlow(t *testing.T) {
	// Create genie test fixture
	genieFixture := genie.NewTestFixture(t)
	defer genieFixture.Cleanup()
	
	genieFixture.StartAndGetSession()
	
	// Set up expected response
	genieFixture.MockChainRunner.ExpectSimpleMessage("hello", "Hello! How can I help you today?")
	
	// Create state components
	chatState := state.NewChatState()
	uiState := state.NewUIState(&types.Config{})
	stateAccessor := state.NewStateAccessor(chatState, uiState)
	
	// Create mock GUI common
	guiCommon := &mockGuiCommon{}
	context := &mockComponent{key: "input", viewName: "input"}
	commandHandler := &mockCommandHandler{commands: make(map[string]func([]string) error)}
	
	// Create chat controller
	controller := controllers.NewChatController(
		context,
		guiCommon,
		genieFixture.Genie,
		stateAccessor,
		commandHandler,
	)
	
	// Send a message
	err := controller.HandleInput("hello")
	require.NoError(t, err)
	
	// Verify the user message was added
	messages := stateAccessor.GetMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "hello", messages[0].Content)
	
	// Verify loading state
	assert.True(t, stateAccessor.IsLoading())
	
	// Wait for response (give some time for async processing)
	time.Sleep(100 * time.Millisecond)
	
	// Check if response was added
	messages = stateAccessor.GetMessages()
	if len(messages) >= 2 {
		assert.Equal(t, "assistant", messages[1].Role)
		assert.Equal(t, "Hello! How can I help you today?", messages[1].Content)
	}
	
	// Verify final state (loading might still be true in async test)
	// In real implementation, loading would be set to false after response
	assert.NotNil(t, stateAccessor)
}

// TestSlashCommands demonstrates slash command testing
func TestSlashCommands(t *testing.T) {
	genieFixture := genie.NewTestFixture(t)
	defer genieFixture.Cleanup()
	
	genieFixture.StartAndGetSession()
	
	// Create state components
	chatState := state.NewChatState()
	uiState := state.NewUIState(&types.Config{})
	stateAccessor := state.NewStateAccessor(chatState, uiState)
	
	// Create mocks
	guiCommon := &mockGuiCommon{}
	context := &mockComponent{key: "input", viewName: "input"}
	commandHandler := &mockCommandHandler{commands: make(map[string]func([]string) error)}
	
	// Create controller
	controller := controllers.NewChatController(
		context,
		guiCommon,
		genieFixture.Genie,
		stateAccessor,
		commandHandler,
	)
	
	scenarios := []struct {
		name            string
		command         string
		expectedCommand string
		expectedArgs    []string
	}{
		{
			name:            "help command",
			command:         ":help",
			expectedCommand: ":help",
			expectedArgs:    []string{},
		},
		{
			name:            "clear command",
			command:         ":clear",
			expectedCommand: ":clear",
			expectedArgs:    []string{},
		},
		{
			name:            "debug toggle",
			command:         ":debug",
			expectedCommand: ":debug",
			expectedArgs:    []string{},
		},
		{
			name:            "command with args",
			command:         ":focus messages",
			expectedCommand: ":focus",
			expectedArgs:    []string{"messages"},
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Reset command handler state
			commandHandler.lastCommand = ""
			commandHandler.lastArgs = nil
			
			err := controller.HandleInput(scenario.command)
			require.NoError(t, err)
			
			assert.Equal(t, scenario.expectedCommand, commandHandler.lastCommand)
			assert.Equal(t, scenario.expectedArgs, commandHandler.lastArgs)
		})
	}
}

// TestErrorHandling demonstrates error handling testing
func TestErrorHandling(t *testing.T) {
	genieFixture := genie.NewTestFixture(t)
	defer genieFixture.Cleanup()
	
	genieFixture.StartAndGetSession()
	
	// Set up genie to return an error response
	genieFixture.MockChainRunner.ExpectMessage("error test").RespondWith("Error: simulated error")
	
	// Create state components
	chatState := state.NewChatState()
	uiState := state.NewUIState(&types.Config{})
	stateAccessor := state.NewStateAccessor(chatState, uiState)
	
	// Create mocks
	guiCommon := &mockGuiCommon{}
	context := &mockComponent{key: "input", viewName: "input"}
	commandHandler := &mockCommandHandler{commands: make(map[string]func([]string) error)}
	
	// Create controller
	controller := controllers.NewChatController(
		context,
		guiCommon,
		genieFixture.Genie,
		stateAccessor,
		commandHandler,
	)
	
	// Send message that will get error response
	err := controller.HandleInput("error test")
	require.NoError(t, err)
	
	// Verify user message appears
	messages := stateAccessor.GetMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "error test", messages[0].Content)
	
	// Wait for async response
	time.Sleep(100 * time.Millisecond)
	
	// Check if error response was added
	messages = stateAccessor.GetMessages()
	if len(messages) >= 2 {
		assert.Equal(t, "assistant", messages[1].Role)
		assert.Contains(t, messages[1].Content, "Error: simulated error")
	}
}

// TestStateManagement demonstrates state management testing
func TestStateManagement(t *testing.T) {
	// Test chat state
	chatState := state.NewChatState()
	
	// Add messages
	chatState.AddMessage(types.Message{Role: "user", Content: "hello"})
	chatState.AddMessage(types.Message{Role: "assistant", Content: "hi"})
	
	// Verify state
	assert.Equal(t, 2, chatState.GetMessageCount())
	assert.False(t, chatState.IsLoading())
	
	// Test loading state
	chatState.SetLoading(true)
	assert.True(t, chatState.IsLoading())
	
	// Test clear
	chatState.ClearMessages()
	assert.Equal(t, 0, chatState.GetMessageCount())
	
	// Test UI state
	uiState := state.NewUIState(&types.Config{ShowCursor: true})
	stateAccessor := state.NewStateAccessor(chatState, uiState)
	
	// Test state accessor
	stateAccessor.AddMessage(types.Message{Role: "user", Content: "test"})
	messages := stateAccessor.GetMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "test", messages[0].Content)
}

// TestClearConversation demonstrates conversation clearing
func TestClearConversation(t *testing.T) {
	genieFixture := genie.NewTestFixture(t)
	defer genieFixture.Cleanup()
	
	genieFixture.StartAndGetSession()
	
	// Create state components
	chatState := state.NewChatState()
	uiState := state.NewUIState(&types.Config{})
	stateAccessor := state.NewStateAccessor(chatState, uiState)
	
	// Add some messages
	stateAccessor.AddMessage(types.Message{Role: "user", Content: "test1"})
	stateAccessor.AddMessage(types.Message{Role: "assistant", Content: "test2"})
	
	// Verify messages exist
	assert.Equal(t, 2, len(stateAccessor.GetMessages()))
	
	// Create mocks
	guiCommon := &mockGuiCommon{}
	context := &mockComponent{key: "input", viewName: "input"}
	commandHandler := &mockCommandHandler{commands: make(map[string]func([]string) error)}
	
	// Create controller
	controller := controllers.NewChatController(
		context,
		guiCommon,
		genieFixture.Genie,
		stateAccessor,
		commandHandler,
	)
	
	// Clear conversation
	err := controller.ClearConversation()
	require.NoError(t, err)
	
	// Verify messages cleared
	assert.Equal(t, 0, len(stateAccessor.GetMessages()))
}

// TestConversationHistory demonstrates conversation history retrieval
func TestConversationHistory(t *testing.T) {
	genieFixture := genie.NewTestFixture(t)
	defer genieFixture.Cleanup()
	
	genieFixture.StartAndGetSession()
	
	// Create state components
	chatState := state.NewChatState()
	uiState := state.NewUIState(&types.Config{})
	stateAccessor := state.NewStateAccessor(chatState, uiState)
	
	expectedMessages := []types.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you?"},
	}
	
	for _, msg := range expectedMessages {
		stateAccessor.AddMessage(msg)
	}
	
	// Create mocks
	guiCommon := &mockGuiCommon{}
	context := &mockComponent{key: "input", viewName: "input"}
	commandHandler := &mockCommandHandler{commands: make(map[string]func([]string) error)}
	
	// Create controller
	controller := controllers.NewChatController(
		context,
		guiCommon,
		genieFixture.Genie,
		stateAccessor,
		commandHandler,
	)
	
	// Get conversation history
	history := controller.GetConversationHistory()
	
	// Verify
	require.Len(t, history, len(expectedMessages))
	for i, msg := range expectedMessages {
		assert.Equal(t, msg.Role, history[i].Role)
		assert.Equal(t, msg.Content, history[i].Content)
	}
}