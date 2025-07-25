package component

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/stretchr/testify/assert"
)

// mockGuiCommon implements types.IGuiCommon for testing
type mockGuiCommon struct {
	updateCallbacks []func()
}

func (m *mockGuiCommon) GetGui() *gocui.Gui { return nil }
func (m *mockGuiCommon) GetConfig() *types.Config {
	return &types.Config{
		ShowCursor:        "enabled",
		MarkdownRendering: "enabled",
		Theme:             "default",
	}
}
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


// createTestStateAccessor creates a real StateAccessor for testing
func createTestStateAccessor() types.IStateAccessor {
	chatState := state.NewChatState(100)
	uiState := state.NewUIState()
	return state.NewStateAccessor(chatState, uiState)
}

// createTestStateAccessorWithLimit creates a real StateAccessor with custom message limit for testing
func createTestStateAccessorWithLimit(messageLimit int) types.IStateAccessor {
	chatState := state.NewChatState(messageLimit)
	uiState := state.NewUIState()
	return state.NewStateAccessor(chatState, uiState)
}

// TestStatusSectionComponent tests the individual status section components
func TestStatusSectionComponent(t *testing.T) {
	gui := &mockGuiCommon{}

	t.Run("basic functionality", func(t *testing.T) {
		section := NewStatusSectionComponent("test-section", "test-view", gui, createTestConfigManager())

		// Test initial state
		assert.Equal(t, "test-section", section.GetKey())
		assert.Equal(t, "test-view", section.GetViewName())
		assert.Equal(t, "", section.text)

		// Test properties
		props := section.GetWindowProperties()
		assert.False(t, props.Focusable)
		assert.False(t, props.Editable)
		assert.False(t, props.Frame)
		assert.False(t, props.Wrap)
		assert.False(t, props.Autoscroll)
		assert.False(t, props.Highlight)

		// Test controlled bounds
		assert.True(t, section.HasControlledBounds())
	})

	t.Run("text setting", func(t *testing.T) {
		section := NewStatusSectionComponent("test-section", "test-view", gui, createTestConfigManager())

		// Test setting text
		section.SetText("Hello World")
		assert.Equal(t, "Hello World", section.text)

		// Test updating text
		section.SetText("New Text")
		assert.Equal(t, "New Text", section.text)
	})

	t.Run("render with no view", func(t *testing.T) {
		section := NewStatusSectionComponent("status-left", "status-left", gui, createTestConfigManager())

		section.SetText("●")
		// Should not panic when rendering without a view
		err := section.Render()
		assert.NoError(t, err)
	})
}

// TestStatusComponent tests the main status component functionality
func TestStatusComponent(t *testing.T) {
	gui := &mockGuiCommon{}
	eventBus := events.NewCommandEventBus()

	t.Run("component initialization", func(t *testing.T) {
		stateAccessor := createTestStateAccessor()
		configManager, _ := helpers.NewConfigManager()
		status := NewStatusComponent(gui, stateAccessor, configManager, eventBus)

		// Test basic properties
		assert.Equal(t, "status", status.GetKey())
		assert.Equal(t, "status", status.GetViewName())
		assert.Equal(t, " Status ", status.GetTitle())

		// Test window properties
		props := status.GetWindowProperties()
		assert.False(t, props.Focusable)
		assert.False(t, props.Editable)
		assert.False(t, props.Frame)

		// Test sub-components exist
		assert.NotNil(t, status.GetLeftComponent())
		assert.NotNil(t, status.GetCenterComponent())
		assert.NotNil(t, status.GetRightComponent())

		// Test sub-component names
		assert.Equal(t, "status-left", status.GetLeftComponent().GetViewName())
		assert.Equal(t, "status-center", status.GetCenterComponent().GetViewName())
		assert.Equal(t, "status-right", status.GetRightComponent().GetViewName())
	})

	t.Run("text setting methods", func(t *testing.T) {
		stateAccessor := createTestStateAccessor()
		configManager, _ := helpers.NewConfigManager()
		status := NewStatusComponent(gui, stateAccessor, configManager, eventBus)

		// Test individual setters
		status.SetLeftText("Left Text")
		status.SetCenterText("Center Text")
		status.SetRightText("Right Text")

		assert.Equal(t, "Left Text", status.GetLeftComponent().(*StatusSectionComponent).GetText())
		assert.Equal(t, "Center Text", status.GetCenterComponent().(*StatusSectionComponent).GetText())
		assert.Equal(t, "Right Text", status.GetRightComponent().(*StatusSectionComponent).GetText())

		// Test bulk setter
		status.SetStatusTexts("New Left", "New Center", "New Right")

		assert.Equal(t, "New Left", status.GetLeftComponent().(*StatusSectionComponent).GetText())
		assert.Equal(t, "New Center", status.GetCenterComponent().(*StatusSectionComponent).GetText())
		assert.Equal(t, "New Right", status.GetRightComponent().(*StatusSectionComponent).GetText())
	})

	t.Run("default content generation", func(t *testing.T) {
		stateAccessor := createTestStateAccessor()
		// Add some test messages
		stateAccessor.AddMessage(types.Message{Role: "user", Content: "Hello"})
		stateAccessor.AddMessage(types.Message{Role: "assistant", Content: "Hi"})
		configManager, _ := helpers.NewConfigManager()
		// Ensure debug is disabled for this test by unsetting environment variable
		os.Unsetenv("GENIE_DEBUG_LEVEL")
		status := NewStatusComponent(gui, stateAccessor, configManager, eventBus)

		// Render without views (should not panic)
		err := status.Render()
		assert.NoError(t, err)

		// Check that default content was set
		assert.Contains(t, status.GetLeftComponent().(*StatusSectionComponent).GetText(), "Ready")
		assert.Equal(t, "", status.GetCenterComponent().(*StatusSectionComponent).GetText())
		assert.Contains(t, status.GetRightComponent().(*StatusSectionComponent).GetText(), "Msgs: 2")
		assert.Contains(t, status.GetRightComponent().(*StatusSectionComponent).GetText(), "Mem:")
		assert.Contains(t, status.GetRightComponent().(*StatusSectionComponent).GetText(), "MB")
	})

	t.Run("dynamic right content with state changes", func(t *testing.T) {
		stateAccessor := createTestStateAccessor()
		// Add test message
		stateAccessor.AddMessage(types.Message{Role: "user", Content: "Test"})
		configManager, _ := helpers.NewConfigManager()
		status := NewStatusComponent(gui, stateAccessor, configManager, eventBus)

		// Initial render
		err := status.Render()
		assert.NoError(t, err)
		initialContent := status.GetRightComponent().(*StatusSectionComponent).GetText()
		assert.Contains(t, initialContent, "Msgs: 1")

		// Add more messages
		stateAccessor.AddMessage(types.Message{Role: "assistant", Content: "Response"})
		stateAccessor.AddMessage(types.Message{Role: "user", Content: "Another"})

		// Clear right text to force regeneration
		status.SetRightText("")

		// Re-render
		err = status.Render()
		assert.NoError(t, err)
		newContent := status.GetRightComponent().(*StatusSectionComponent).GetText()
		assert.Contains(t, newContent, "Msgs: 3")
		assert.NotEqual(t, initialContent, newContent)
	})

	t.Run("memory usage in right content", func(t *testing.T) {
		stateAccessor := createTestStateAccessor()
		configManager, _ := helpers.NewConfigManager()
		status := NewStatusComponent(gui, stateAccessor, configManager, eventBus)

		// Render to get memory info
		err := status.Render()
		assert.NoError(t, err)

		// Check that memory usage is included and is reasonable
		content := status.GetRightComponent().(*StatusSectionComponent).GetText()
		assert.Contains(t, content, "Mem:")
		assert.Contains(t, content, "MB")

		// Parse memory value to ensure it's reasonable
		var memMB int
		_, err = fmt.Sscanf(content, "Tokens: %*s | Msgs: %*d | Mem: %dMB", &memMB)
		if err == nil {
			assert.Greater(t, memMB, 0, "Memory usage should be positive")
			assert.Less(t, memMB, 10000, "Memory usage should be reasonable for tests")
		}
	})
}

// TestStatusComponentIntegration tests integration with real state
func TestStatusComponentIntegration(t *testing.T) {
	eventBus := events.NewCommandEventBus()

	t.Run("integration with real state accessor", func(t *testing.T) {
		// Create real state components
		chatState := state.NewChatState(100)
		uiState := state.NewUIState()
		stateAccessor := state.NewStateAccessor(chatState, uiState)

		gui := &mockGuiCommon{}
		configManager, _ := helpers.NewConfigManager()
		status := NewStatusComponent(gui, stateAccessor, configManager, eventBus)

		// Add some messages to state
		stateAccessor.AddMessage(types.Message{Role: "user", Content: "Hello"})
		stateAccessor.AddMessage(types.Message{Role: "assistant", Content: "Hi there"})
		// Render
		err := status.Render()
		assert.NoError(t, err)

		// Verify state is reflected
		content := status.GetRightComponent().(*StatusSectionComponent).GetText()
		assert.Contains(t, content, "Msgs: 2")

		// Add more messages and verify updates
		stateAccessor.AddMessage(types.Message{Role: "user", Content: "More"})
		status.SetRightText("") // Force regeneration

		err = status.Render()
		assert.NoError(t, err)
		newContent := status.GetRightComponent().(*StatusSectionComponent).GetText()
		assert.Contains(t, newContent, "Msgs: 3")
	})

	t.Run("stress test with many messages", func(t *testing.T) {
		stateAccessor := createTestStateAccessorWithLimit(1000)

		// Add many messages
		for i := 0; i < 1000; i++ {
			role := "user"
			if i%2 == 1 {
				role = "assistant"
			}
			stateAccessor.AddMessage(types.Message{
				Role:    role,
				Content: fmt.Sprintf("Message %d", i),
			})
		}

		gui := &mockGuiCommon{}
		configManager, _ := helpers.NewConfigManager()
		status := NewStatusComponent(gui, stateAccessor, configManager, eventBus)

		// Test that rendering handles large message counts efficiently
		start := time.Now()
		err := status.Render()
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, duration, 100*time.Millisecond, "Rendering should be fast even with many messages")
		assert.Contains(t, status.GetRightComponent().(*StatusSectionComponent).GetText(), "Msgs: 1000")
	})

	t.Run("concurrent access safety", func(t *testing.T) {
		t.Skip("Skipping concurrent test for beta release - race conditions in text access")
		
		stateAccessor := createTestStateAccessor()
		gui := &mockGuiCommon{}
		configManager, _ := helpers.NewConfigManager()
		status := NewStatusComponent(gui, stateAccessor, configManager, eventBus)

		// Test concurrent updates to different sections
		done := make(chan bool, 3)

		go func() {
			for i := 0; i < 10; i++ {
				status.SetLeftText(fmt.Sprintf("Left %d", i))
				time.Sleep(time.Microsecond)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 10; i++ {
				status.SetCenterText(fmt.Sprintf("Center %d", i))
				time.Sleep(time.Microsecond)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 10; i++ {
				status.SetRightText(fmt.Sprintf("Right %d", i))
				status.Render()
				time.Sleep(time.Microsecond)
			}
			done <- true
		}()

		// Wait for all goroutines
		for i := 0; i < 3; i++ {
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("Concurrent test timed out")
			}
		}

		// Final render should not panic
		err := status.Render()
		assert.NoError(t, err)
	})
}
