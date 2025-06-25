package history

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChatHistory(t *testing.T) {
	t.Run("with saving enabled", func(t *testing.T) {
		history := NewChatHistory("/test/path", true).(*FileChatHistory)
		assert.Equal(t, "/test/path", history.filePath)
		assert.True(t, history.saveEnabled)
		assert.Equal(t, 50, history.maxSize)
		assert.Equal(t, -1, history.currentIndex)
		assert.Empty(t, history.commands)
	})

	t.Run("with saving disabled", func(t *testing.T) {
		history := NewChatHistory("/test/path", false).(*FileChatHistory)
		assert.Equal(t, "/test/path", history.filePath)
		assert.False(t, history.saveEnabled)
		assert.Equal(t, 50, history.maxSize)
		assert.Equal(t, -1, history.currentIndex)
		assert.Empty(t, history.commands)
	})
}

func TestChatHistory_AddCommand(t *testing.T) {
	history := NewChatHistory("", false) // No saving for tests

	t.Run("adds single command", func(t *testing.T) {
		history.AddCommand("hello")
		commands := history.GetHistory()
		assert.Equal(t, []string{"hello"}, commands)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		history := NewChatHistory("", false)
		history.AddCommand("  hello world  ")
		commands := history.GetHistory()
		assert.Equal(t, []string{"hello world"}, commands)
	})

	t.Run("ignores empty commands", func(t *testing.T) {
		history := NewChatHistory("", false)
		history.AddCommand("")
		history.AddCommand("   ")
		commands := history.GetHistory()
		assert.Empty(t, commands)
	})

	t.Run("removes duplicates", func(t *testing.T) {
		history := NewChatHistory("", false)
		history.AddCommand("hello")
		history.AddCommand("world")
		history.AddCommand("hello") // duplicate
		commands := history.GetHistory()
		assert.Equal(t, []string{"world", "hello"}, commands)
	})

	t.Run("maintains max size", func(t *testing.T) {
		history := NewChatHistory("", false).(*FileChatHistory)
		history.maxSize = 3 // Set small max for testing

		// Add more commands than max size
		for i := 1; i <= 5; i++ {
			history.AddCommand(fmt.Sprintf("command%d", i))
		}

		commands := history.GetHistory()
		assert.Len(t, commands, 3)
		assert.Equal(t, []string{"command3", "command4", "command5"}, commands)
	})

	t.Run("resets navigation after adding", func(t *testing.T) {
		history := NewChatHistory("", false).(*FileChatHistory)
		history.AddCommand("first")
		history.NavigatePrev() // Navigate to first command
		assert.Equal(t, 0, history.currentIndex)

		history.AddCommand("second") // Should reset navigation
		assert.Equal(t, -1, history.currentIndex)
	})
}

func TestChatHistory_Navigation_EmptyHistory(t *testing.T) {
	history := NewChatHistory("", false)

	t.Run("NavigatePrev returns empty on empty history", func(t *testing.T) {
		result := history.NavigatePrev()
		assert.Equal(t, "", result)
	})

	t.Run("NavigateNext returns empty on empty history", func(t *testing.T) {
		result := history.NavigateNext()
		assert.Equal(t, "", result)
	})

	t.Run("ResetNavigation returns empty", func(t *testing.T) {
		result := history.ResetNavigation()
		assert.Equal(t, "", result)
	})
}

func TestChatHistory_Navigation_SingleCommand(t *testing.T) {
	history := NewChatHistory("", false)
	history.AddCommand("only command")

	t.Run("NavigatePrev returns the command", func(t *testing.T) {
		result := history.NavigatePrev()
		assert.Equal(t, "only command", result)
	})

	t.Run("NavigatePrev again stays on same command", func(t *testing.T) {
		history.NavigatePrev() // First call
		result := history.NavigatePrev() // Second call
		assert.Equal(t, "only command", result)
	})

	t.Run("NavigateNext from command returns empty", func(t *testing.T) {
		history.NavigatePrev() // Go to command
		result := history.NavigateNext() // Go forward (to end)
		assert.Equal(t, "", result)
	})

	t.Run("NavigateNext again stays empty", func(t *testing.T) {
		history.NavigatePrev() // Go to command
		history.NavigateNext() // Go to end
		result := history.NavigateNext() // Try to go beyond end
		assert.Equal(t, "", result)
	})
}

func TestChatHistory_Navigation_MultipleCommands(t *testing.T) {
	history := NewChatHistory("", false)
	// Add commands: most recent is "third", oldest is "first"
	history.AddCommand("first")
	history.AddCommand("second") 
	history.AddCommand("third")

	t.Run("NavigatePrev sequence - newest to oldest", func(t *testing.T) {
		// Start at end (-1), navigate backwards through history
		result1 := history.NavigatePrev() // Should get most recent
		assert.Equal(t, "third", result1)

		result2 := history.NavigatePrev() // Should get second most recent
		assert.Equal(t, "second", result2)

		result3 := history.NavigatePrev() // Should get oldest
		assert.Equal(t, "first", result3)

		result4 := history.NavigatePrev() // Should stay at oldest
		assert.Equal(t, "first", result4)
	})

	t.Run("NavigateNext sequence - oldest to newest", func(t *testing.T) {
		// Start from oldest and navigate forward
		history.NavigatePrev() // third
		history.NavigatePrev() // second
		history.NavigatePrev() // first (oldest)

		result1 := history.NavigateNext() // Should get second
		assert.Equal(t, "second", result1)

		result2 := history.NavigateNext() // Should get third (newest)
		assert.Equal(t, "third", result2)

		result3 := history.NavigateNext() // Should go to end (empty)
		assert.Equal(t, "", result3)

		result4 := history.NavigateNext() // Should stay at end
		assert.Equal(t, "", result4)
	})

	t.Run("Mixed navigation", func(t *testing.T) {
		// Reset to end
		history.ResetNavigation()

		// Go back two steps
		history.NavigatePrev() // third
		result1 := history.NavigatePrev() // second
		assert.Equal(t, "second", result1)

		// Go forward one step
		result2 := history.NavigateNext() // third
		assert.Equal(t, "third", result2)

		// Go back one step
		result3 := history.NavigatePrev() // second
		assert.Equal(t, "second", result3)

		// Go forward two steps (to end)
		history.NavigateNext() // third
		result4 := history.NavigateNext() // end
		assert.Equal(t, "", result4)
	})
}

func TestChatHistory_ResetNavigation(t *testing.T) {
	history := NewChatHistory("", false)
	history.AddCommand("first")
	history.AddCommand("second")

	// Navigate to a command
	history.NavigatePrev()
	historyImpl := history.(*FileChatHistory)
	assert.Equal(t, 0, historyImpl.currentIndex) // Should be at "second"

	// Reset navigation
	result := history.ResetNavigation()
	assert.Equal(t, "", result)
	assert.Equal(t, -1, historyImpl.currentIndex) // Should be at end

	// Next NavigatePrev should get most recent
	nextResult := history.NavigatePrev()
	assert.Equal(t, "second", nextResult)
}

func TestChatHistory_NavigationIndexBoundaries(t *testing.T) {
	history := NewChatHistory("", false)
	historyImpl := history.(*FileChatHistory)
	
	// Add test commands
	history.AddCommand("first")
	history.AddCommand("second")
	history.AddCommand("third")

	t.Run("index boundaries on NavigatePrev", func(t *testing.T) {
		// Start at -1 (end)
		assert.Equal(t, -1, historyImpl.currentIndex)

		history.NavigatePrev() // index 0 ("third")
		assert.Equal(t, 0, historyImpl.currentIndex)

		history.NavigatePrev() // index 1 ("second")
		assert.Equal(t, 1, historyImpl.currentIndex)

		history.NavigatePrev() // index 2 ("first")
		assert.Equal(t, 2, historyImpl.currentIndex)

		history.NavigatePrev() // should stay at index 2
		assert.Equal(t, 2, historyImpl.currentIndex)
	})

	t.Run("index boundaries on NavigateNext", func(t *testing.T) {
		// Start at oldest (index 2)
		historyImpl.currentIndex = 2

		history.NavigateNext() // index 1 ("second")
		assert.Equal(t, 1, historyImpl.currentIndex)

		history.NavigateNext() // index 0 ("third")
		assert.Equal(t, 0, historyImpl.currentIndex)

		history.NavigateNext() // index -1 (end)
		assert.Equal(t, -1, historyImpl.currentIndex)

		history.NavigateNext() // should stay at -1
		assert.Equal(t, -1, historyImpl.currentIndex)
	})
}

func TestChatHistory_SaveLoad_Disabled(t *testing.T) {
	history := NewChatHistory("/tmp/test", false)

	t.Run("Save does nothing when disabled", func(t *testing.T) {
		err := history.Save()
		assert.NoError(t, err) // Should not error, just do nothing
	})

	t.Run("Load does nothing when disabled", func(t *testing.T) {
		err := history.Load()
		assert.NoError(t, err) // Should not error, just do nothing
	})
}

func TestChatHistory_GetHistory(t *testing.T) {
	history := NewChatHistory("", false)
	history.AddCommand("first")
	history.AddCommand("second")

	t.Run("returns copy of commands", func(t *testing.T) {
		commands1 := history.GetHistory()
		commands2 := history.GetHistory()

		// Should be equal but different slices
		assert.Equal(t, commands1, commands2)
		assert.NotSame(t, &commands1[0], &commands2[0]) // Different underlying arrays
	})

	t.Run("modifying returned slice doesn't affect original", func(t *testing.T) {
		commands := history.GetHistory()
		commands[0] = "modified"

		// Original should be unchanged
		originalCommands := history.GetHistory()
		assert.Equal(t, []string{"first", "second"}, originalCommands)
	})
}