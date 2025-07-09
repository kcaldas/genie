package testing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestHelp tests the help command functionality
func TestHelp(t *testing.T) {
	driver := NewTUIDriver(t)
	defer driver.Close()

	// Wait for app to initialize
	driver.Wait()

	t.Run("help command shows help content", func(t *testing.T) {
		// Ensure input is focused
		driver.FocusInput()

		// Initially help should not be visible
		assert.False(t, driver.Help().IsVisible(), "Help should not be visible initially")

		// Type :help and press enter
		driver.Input().Type(":help").PressEnter()
		driver.WaitFor(50 * time.Millisecond)

		// Help panel should now be visible
		assert.True(t, driver.Help().IsVisible(), "Help should be visible after :help command")

		// Content should be present and contain expected text
		content := driver.Help().GetContent()
		assert.NotEmpty(t, content, "Help content should not be empty")
		assert.Contains(t, content, "GENIE", "Help content should contain GENIE")
	})

	t.Run("help command toggles off", func(t *testing.T) {
		// Assume help is currently visible from previous test
		// (or we could explicitly show it first)

		// Type :help again to toggle off
		driver.Input().TypeAndEnter(":help")

		// Wait for async event processing to complete
		driver.Wait()

		// Help panel should now be hidden
		assert.False(t, driver.Help().IsVisible(), "Help should be hidden after second :help command")
	})

	t.Run("F1 key shows help", func(t *testing.T) {
		// Press F1 (should be mapped to help)
		driver.Layout().PressF1()

		// Wait for async event processing to complete
		driver.Wait()

		// Help should be visible
		assert.True(t, driver.Help().IsVisible(), "Help should be visible after F1 key")

		// Content should be present
		content := driver.Help().GetContent()
		assert.NotEmpty(t, content, "Help content should not be empty")
	})
}
