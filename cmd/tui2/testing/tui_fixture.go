package testing

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/cmd/tui2"
	"github.com/kcaldas/genie/cmd/tui2/types"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/stretchr/testify/require"
)

// TUIFixture provides a complete testing setup for the TUI with mocked dependencies
type TUIFixture struct {
	*genie.TestFixture // Embed the core genie test fixture
	App               *tui2.App
	Driver            *TUIDriver
	t                 *testing.T
}

// TUIFixtureOption allows customization of the TUI test fixture
type TUIFixtureOption func(*TUIFixture)

// NewTUIFixture creates a new TUI testing fixture building on the core genie fixture
func NewTUIFixture(t *testing.T, opts ...TUIFixtureOption) *TUIFixture {
	t.Helper()

	// Create the base genie test fixture
	genieFixture := genie.NewTestFixture(t)

	// Create the TUI app with the test genie instance
	app, err := tui2.NewApp(genieFixture.Genie)
	require.NoError(t, err)

	fixture := &TUIFixture{
		TestFixture: genieFixture,
		App:         app,
		t:           t,
	}

	// Create TUI driver for testing interactions
	fixture.Driver = NewTUIDriver(app, genieFixture.EventBus, t)

	// Apply any custom options
	for _, opt := range opts {
		opt(fixture)
	}

	return fixture
}

// WithHeadlessMode configures the TUI for headless testing (no actual GUI)
func WithHeadlessMode() TUIFixtureOption {
	return func(f *TUIFixture) {
		// Configure for headless mode - we'll implement this
		// when we need actual GUI interaction testing
	}
}

// WithRealGUI configures the TUI for visual testing (actual GUI display)
func WithRealGUI() TUIFixtureOption {
	return func(f *TUIFixture) {
		// Configure for real GUI testing
	}
}

// StartTUI starts the TUI application for testing
func (f *TUIFixture) StartTUI() {
	f.t.Helper()

	// Start the underlying genie service
	f.StartAndGetSession()

	// Initialize TUI components but don't start the main loop
	// (for testing we control the flow manually)
}

// SendMessage simulates user typing a message and pressing enter
func (f *TUIFixture) SendMessage(message string) {
	f.t.Helper()
	f.Driver.Input().Type(message).PressEnter()
}

// SendCommand simulates user typing a slash command
func (f *TUIFixture) SendCommand(command string) {
	f.t.Helper()
	f.Driver.Input().Type(command).PressEnter()
}

// ExpectMessageInChat waits for a message to appear in the chat
func (f *TUIFixture) ExpectMessageInChat(role, content string, timeout time.Duration) {
	f.t.Helper()

	f.Driver.Messages().Eventually(func() bool {
		messages := f.Driver.Messages().GetMessages()
		for _, msg := range messages {
			if msg.Role == role && msg.Content == content {
				return true
			}
		}
		return false
	}, timeout, "Expected message not found in chat: role=%s, content=%s", role, content)
}

// ExpectLoading waits for the loading state to be active
func (f *TUIFixture) ExpectLoading(timeout time.Duration) {
	f.t.Helper()

	f.Driver.Messages().Eventually(func() bool {
		return f.Driver.Messages().IsLoading()
	}, timeout, "Expected loading state not found")
}

// ExpectNotLoading waits for the loading state to be inactive
func (f *TUIFixture) ExpectNotLoading(timeout time.Duration) {
	f.t.Helper()

	f.Driver.Messages().Eventually(func() bool {
		return !f.Driver.Messages().IsLoading()
	}, timeout, "Expected loading to stop")
}

// SwitchToPanel changes focus to a specific panel
func (f *TUIFixture) SwitchToPanel(panel string) {
	f.t.Helper()
	f.SendCommand("/focus " + panel)
}

// ToggleDebugPanel toggles the debug panel visibility
func (f *TUIFixture) ToggleDebugPanel() {
	f.t.Helper()
	f.SendCommand("/debug")
}

// ExpectDebugMessage waits for a debug message to appear
func (f *TUIFixture) ExpectDebugMessage(content string, timeout time.Duration) {
	f.t.Helper()

	f.Driver.Debug().Eventually(func() bool {
		messages := f.Driver.Debug().GetMessages()
		for _, msg := range messages {
			if msg == content {
				return true
			}
		}
		return false
	}, timeout, "Expected debug message not found: %s", content)
}

// GetLastMessage returns the last message in the chat
func (f *TUIFixture) GetLastMessage() *types.Message {
	messages := f.Driver.Messages().GetMessages()
	if len(messages) == 0 {
		return nil
	}
	return &messages[len(messages)-1]
}

// AssertMessageCount checks the total number of messages
func (f *TUIFixture) AssertMessageCount(expected int) {
	f.t.Helper()
	messages := f.Driver.Messages().GetMessages()
	require.Len(f.t, messages, expected, "Unexpected message count")
}

// AssertLastMessage checks the last message content and role
func (f *TUIFixture) AssertLastMessage(expectedRole, expectedContent string) {
	f.t.Helper()
	lastMsg := f.GetLastMessage()
	require.NotNil(f.t, lastMsg, "No messages found")
	require.Equal(f.t, expectedRole, lastMsg.Role, "Unexpected message role")
	require.Equal(f.t, expectedContent, lastMsg.Content, "Unexpected message content")
}

// SimulateEvent publishes an event to the event bus for testing
func (f *TUIFixture) SimulateEvent(topic string, event interface{}) {
	f.EventBus.Publish(topic, event)
}

// Wait provides a simple wait mechanism for testing
func (f *TUIFixture) Wait(duration time.Duration) {
	time.Sleep(duration)
}

// Cleanup cleans up the TUI fixture (calls parent cleanup)
func (f *TUIFixture) Cleanup() {
	if f.App != nil {
		f.App.Close()
	}
	f.TestFixture.Cleanup()
}