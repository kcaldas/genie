package testing

import (
	"strings"
	"testing"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/stretchr/testify/require"
)

// TUIDriver provides a high-level interface for testing TUI interactions
type TUIDriver struct {
	testingScreen   gocui.TestingScreen
	app             *tui.App
	gui             *gocui.Gui
	cleanup         func()
	t               *testing.T
	commandEventBus *events.CommandEventBus
}

// NewTUIDriver creates a new TUI driver for testing
func NewTUIDriver(t *testing.T) *TUIDriver {
	// Create genie test fixture
	genieFixture := genie.NewTestFixture(t)
	session := genieFixture.StartAndGetSession()

	// Create command event bus
	commandEventBus := events.NewCommandEventBus()

	// Create TUI app with simulator mode for testing
	simulatorMode := gocui.OutputSimulator
	app, err := tui.NewAppWithOutputMode(genieFixture.Genie, session, commandEventBus, &simulatorMode)
	require.NoError(t, err)

	// Get the testing screen from the GUI
	gui := app.GetGui()
	testingScreen := gui.GetTestingScreen()

	// Start the GUI in testing mode
	cleanup := testingScreen.StartGui()

	// Wait a bit for the app to fully initialize
	testingScreen.WaitSync()
	time.Sleep(100 * time.Millisecond)

	return &TUIDriver{
		testingScreen: testingScreen,
		app:           app,
		gui:           gui,
		cleanup: func() {
			cleanup() // Stop the testing GUI
			app.Close()
			genieFixture.Cleanup()
		},
		t:               t,
		commandEventBus: commandEventBus,
	}
}

// Close cleans up the testing driver
func (d *TUIDriver) Close() {
	if d.cleanup != nil {
		d.cleanup()
	}
}

// FocusInput explicitly focuses the input view and ensures it's editable
func (d *TUIDriver) FocusInput() *TUIDriver {
	// Use gui.Update to ensure this runs in the main loop
	d.gui.Update(func(g *gocui.Gui) error {
		view, err := g.SetCurrentView("input")
		if err != nil {
			return err
		}
		// Ensure the view is editable
		view.Editable = true
		// Ensure cursor is visible
		g.Cursor = true
		// Set cursor position to end of current content
		view.SetCursor(len(view.ViewBuffer()), 0)
		return nil
	})
	d.testingScreen.WaitSync()
	return d
}

// Wait waits for async operations and UI updates to complete
func (d *TUIDriver) Wait() *TUIDriver {
	return d.WaitFor(10 * time.Millisecond)
}

// WaitFor waits for a specific duration
func (d *TUIDriver) WaitFor(duration time.Duration) *TUIDriver {
	// First wait for gocui operations to complete
	d.testingScreen.WaitSync()
	// Then wait for any pending event handlers to complete
	d.commandEventBus.WaitForPendingEvents()
	// Finally, give a small amount of time for UI rendering to complete
	// This ensures that UI updates triggered by event handlers are fully rendered
	d.testingScreen.WaitSync()
	time.Sleep(duration)
	return d
}

// Input returns a driver for the input panel
func (d *TUIDriver) Input() *InputDriver {
	return &InputDriver{
		driver: d,
	}
}

// Help returns a driver for the help panel
func (d *TUIDriver) Help() *HelpDriver {
	return &HelpDriver{
		driver: d,
	}
}

// Messages returns a driver for the messages panel
func (d *TUIDriver) Messages() *MessagesDriver {
	return &MessagesDriver{
		driver: d,
	}
}

// Debug returns a driver for the debug panel
func (d *TUIDriver) Debug() *DebugDriver {
	return &DebugDriver{
		driver: d,
	}
}

// Status returns a driver for the status panel
func (d *TUIDriver) Status() *StatusDriver {
	return &StatusDriver{
		driver: d,
	}
}

// Layout returns a driver for layout operations
func (d *TUIDriver) Layout() *LayoutDriver {
	return &LayoutDriver{
		driver: d,
	}
}

// InputDriver provides input panel testing operations
type InputDriver struct {
	driver *TUIDriver
}

// Type simulates typing text into the input field
func (i *InputDriver) Type(text string) *InputDriver {
	// Use the original SendStringAsKeys method followed by WaitSync
	i.driver.testingScreen.SendStringAsKeys(text)
	i.driver.testingScreen.WaitSync()
	return i
}

// PressEnter simulates pressing the enter key
func (i *InputDriver) PressEnter() *InputDriver {
	i.driver.testingScreen.SendKeySync(gocui.KeyEnter)
	return i
}

// TypeAndEnter types text and presses enter
func (i *InputDriver) TypeAndEnter(text string) *InputDriver {
	return i.Type(text).PressEnter()
}

// Clear clears the input field
func (i *InputDriver) Clear() *InputDriver {
	i.driver.testingScreen.SendKey(gocui.KeyCtrlL)
	return i
}

// GetContent returns the current input content
func (i *InputDriver) GetContent() string {
	content, err := i.driver.testingScreen.GetViewContent("input")
	if err != nil {
		return ""
	}
	return content
}

// HelpDriver provides help panel testing operations
type HelpDriver struct {
	driver *TUIDriver
}

// IsVisible returns whether the help panel is visible
func (h *HelpDriver) IsVisible() bool {
	// Check if text-viewer is visible (help uses text-viewer)
	_, err := h.driver.testingScreen.GetViewContent("text-viewer")
	return err == nil // View exists, regardless of content
}

// GetContent returns the help panel content
func (h *HelpDriver) GetContent() string {
	content, err := h.driver.testingScreen.GetViewContent("text-viewer")
	if err != nil {
		return ""
	}
	return content
}

// MessagesDriver provides messages panel testing operations
type MessagesDriver struct {
	driver *TUIDriver
}

// GetContent returns the messages panel content
func (m *MessagesDriver) GetContent() string {
	content, err := m.driver.testingScreen.GetViewContent("messages")
	if err != nil {
		return ""
	}
	return content
}

// GetMessages returns all messages in the chat (parsed from content)
func (m *MessagesDriver) GetMessages() []types.Message {
	// For now, we'll rely on GetContent() and let tests parse it
	// In the future, we could parse the messages view content
	return []types.Message{}
}

// ScrollToTop scrolls to the top of messages
func (m *MessagesDriver) ScrollToTop() *MessagesDriver {
	m.driver.testingScreen.SendKey(gocui.KeyPgup)
	return m
}

// ScrollToBottom scrolls to the bottom of messages
func (m *MessagesDriver) ScrollToBottom() *MessagesDriver {
	m.driver.testingScreen.SendKey(gocui.KeyPgdn)
	return m
}

// Eventually waits for a condition to be true within a timeout
func (m *MessagesDriver) Eventually(condition func() bool, timeout time.Duration, msgAndArgs ...interface{}) {
	start := time.Now()
	for time.Since(start) < timeout {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(msgAndArgs) > 0 {
		m.driver.t.Errorf(msgAndArgs[0].(string), msgAndArgs[1:]...)
	} else {
		m.driver.t.Error("Eventually condition was not met")
	}
}

// IsLoading returns whether the messages panel is showing loading state
func (m *MessagesDriver) IsLoading() bool {
	content := m.GetContent()
	// Look for loading indicators in the content
	return strings.Contains(content, "Loading") || strings.Contains(content, "...")
}

// DebugDriver provides debug panel testing operations
type DebugDriver struct {
	driver *TUIDriver
}

// GetContent returns the debug panel content
func (d *DebugDriver) GetContent() string {
	content, err := d.driver.testingScreen.GetViewContent("debug")
	if err != nil {
		return ""
	}
	return content
}

// IsVisible returns whether the debug panel is visible
func (d *DebugDriver) IsVisible() bool {
	content, err := d.driver.testingScreen.GetViewContent("debug")
	return err == nil && content != ""
}

// GetMessages returns debug messages (parsed from content)
func (d *DebugDriver) GetMessages() []string {
	content := d.GetContent()
	if content == "" {
		return []string{}
	}
	// Simple parsing - split by newlines and filter out empty strings
	lines := strings.Split(content, "\n")
	var messages []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			messages = append(messages, strings.TrimSpace(line))
		}
	}
	return messages
}

// Eventually waits for a condition to be true within a timeout
func (d *DebugDriver) Eventually(condition func() bool, timeout time.Duration, msgAndArgs ...interface{}) {
	start := time.Now()
	for time.Since(start) < timeout {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(msgAndArgs) > 0 {
		d.driver.t.Errorf(msgAndArgs[0].(string), msgAndArgs[1:]...)
	} else {
		d.driver.t.Error("Eventually condition was not met")
	}
}

// StatusDriver provides status panel testing operations
type StatusDriver struct {
	driver *TUIDriver
}

// GetContent returns the status bar content
func (s *StatusDriver) GetContent() string {
	content, err := s.driver.testingScreen.GetViewContent("status")
	if err != nil {
		return ""
	}
	return content
}

// LayoutDriver provides layout testing operations
type LayoutDriver struct {
	driver *TUIDriver
}

// PressTab simulates pressing tab to cycle focus
func (l *LayoutDriver) PressTab() *LayoutDriver {
	l.driver.testingScreen.SendKey(gocui.KeyTab)
	return l
}

// PressF1 simulates pressing F1 (help)
func (l *LayoutDriver) PressF1() *LayoutDriver {
	l.driver.testingScreen.SendKey(gocui.KeyF1)
	return l
}
