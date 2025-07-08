package testing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChat tests the chat functionality
func TestChat(t *testing.T) {
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