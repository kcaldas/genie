package testing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestClearCommand tests the /clear command
func TestClearCommand(t *testing.T) {
	t.Skip("Skipping TUI tests for beta release - contains race conditions in gocui library")
	
	driver := NewTUIDriver(t)
	defer driver.Close()

	// Wait for app to initialize
	driver.Wait()

	t.Run("clears the chat history", func(t *testing.T) {
		// Ensure input is focused
		driver.FocusInput()

		// Send a test message first
		driver.Input().Type("hello").PressEnter()
		driver.WaitFor(100 * time.Millisecond)

		// Check that we have the user message in the view
		messagesBefore := driver.Messages().GetContent()
		assert.Contains(t, messagesBefore, "hello", "Should have user message before clear")

		// Type :clear command
		driver.Input().Type(":clear").PressEnter()
		driver.WaitFor(100 * time.Millisecond)

		// Verify the user message is no longer visible
		messagesAfter := driver.Messages().GetContent()
		assert.NotContains(t, messagesAfter, "hello", "User message should not be visible after clear")
	})
}
