package testing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestChat tests the chat functionality
func TestChat(t *testing.T) {
	t.Skip("Skipping TUI tests for beta release - contains race conditions in gocui library")
	
	driver := NewTUIDriver(t)
	defer driver.Close()

	// Wait for app to initialize
	driver.Wait()

	t.Run("input accepts text", func(t *testing.T) {
		// Ensure input is focused and editable
		driver.FocusInput()

		// Type test text
		driver.Input().Type("test input")
		driver.Wait()

		content := driver.Input().GetContent()
		assert.Contains(t, content, "test input", "Input should contain typed text")

		// Clear for next test
		driver.Input().Clear()
		driver.Wait()
	})
}

// TestChatFlowSingle tests a single message exchange
func TestChatFlowSingle(t *testing.T) {
	t.Skip("Skipping TUI tests for beta release - contains race conditions in gocui library")
	
	driver := NewTUIDriver(t)
	defer driver.Close()

	// Wait for app to initialize
	driver.Wait()

	// Setup mock expectation
	driver.ExpectMessage("hello").RespondWith("Hi there! How can I help you?")

	// Ensure input is focused
	driver.FocusInput()

	// Send user message
	driver.Input().Type("hello").PressEnter()
	driver.WaitFor(100 * time.Millisecond)

	// Verify user message appears in chat
	messages := driver.Messages().GetContent()
	assert.Contains(t, messages, "hello", "User message should appear in chat")

	// Wait a bit longer for AI response to be processed
	driver.WaitFor(200 * time.Millisecond)

	// Verify AI response appears in chat
	messages = driver.Messages().GetContent()
	assert.Contains(t, messages, "Hi there! How can I help you?", "AI response should appear in chat")
}

// TestChatFlowMultiple - multiple message exchange test
func TestChatFlowMultiple(t *testing.T) {
	t.Skip("Skipping TUI tests for beta release - contains race conditions in gocui library")
	
	driver := NewTUIDriver(t)
	defer driver.Close()

	// Wait for app to initialize
	driver.Wait()

	// Setup multiple mock expectations
	driver.ExpectMessage("what is Go").RespondWith("Go is a programming language developed by Google.")
	driver.ExpectMessage("thanks").RespondWith("You're welcome!")

	// Ensure input is focused
	driver.FocusInput()

	// Send first message
	driver.Input().Type("what is Go").PressEnter()
	driver.WaitFor(200 * time.Millisecond)

	// Verify first exchange
	messages := driver.Messages().GetContent()
	assert.Contains(t, messages, "what is Go", "First user message should appear")
	assert.Contains(t, messages, "Go is a programming language", "First AI response should appear")

	// Send second message
	driver.Input().Type("thanks").PressEnter()
	driver.WaitFor(200 * time.Millisecond)

	// Verify second exchange
	messages = driver.Messages().GetContent()
	assert.Contains(t, messages, "thanks", "Second user message should appear")
	assert.Contains(t, messages, "You're welcome!", "Second AI response should appear")
}
