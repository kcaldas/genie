package commands

import (
	"strings"
	"testing"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/stretchr/testify/assert"
)

func createTestHandler() *CommandHandler {
	eventBus := events.NewCommandEventBus()
	mockNotification := &types.MockNotification{}
	return NewCommandHandler(eventBus, mockNotification)
}

func TestVimStyleCommandParsing(t *testing.T) {
	handler := createTestHandler()

	// Mock command function for testing
	var capturedArgs []string
	mockYankHandler := func(args []string) error {
		capturedArgs = args
		return nil
	}

	// Register a yank command
	yankCmd := &mockCommand{
		BaseCommand: BaseCommand{
			Name:        "yank",
			Description: "Copy messages to clipboard",
			Usage:       ":y[count][direction]",
			Aliases:     []string{"y"},
		},
		executeFunc: mockYankHandler,
	}
	handler.RegisterNewCommand(yankCmd)

	tests := []struct {
		name         string
		command      string
		args         []string
		expectCall   bool
		expectedArgs []string
		description  string
	}{
		{
			name:         "simple y command",
			command:      ":y",
			args:         []string{},
			expectCall:   true,
			expectedArgs: []string{},
			description:  "Basic :y should work as before",
		},
		{
			name:         "y with separate args",
			command:      ":y",
			args:         []string{"2k"},
			expectCall:   true,
			expectedArgs: []string{"2k"},
			description:  ":y 2k should work as before",
		},
		{
			name:         "vim-style y1k",
			command:      ":y1k",
			args:         []string{},
			expectCall:   true,
			expectedArgs: []string{"1k"},
			description:  "Vim-style :y1k should parse as :y with arg '1k'",
		},
		{
			name:         "vim-style y5j",
			command:      ":y5j",
			args:         []string{},
			expectCall:   true,
			expectedArgs: []string{"5j"},
			description:  "Vim-style :y5j should parse as :y with arg '5j'",
		},
		{
			name:         "vim-style y3",
			command:      ":y3",
			args:         []string{},
			expectCall:   true,
			expectedArgs: []string{"3"},
			description:  "Vim-style :y3 should parse as :y with arg '3'",
		},
		{
			name:         "vim-style with additional args",
			command:      ":y2k",
			args:         []string{"extra", "args"},
			expectCall:   true,
			expectedArgs: []string{"2k", "extra", "args"},
			description:  "Vim-style with extra args should combine correctly",
		},
		{
			name:         "yank prefix matching",
			command:      ":yank3k",
			args:         []string{},
			expectCall:   true,
			expectedArgs: []string{"3k"},
			description:  "Full 'yank' command should also support vim-style parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset captured args
			capturedArgs = nil

			// Execute command
			err := handler.HandleCommand(tt.command, tt.args)

			if tt.expectCall {
				assert.NoError(t, err, "Command should execute without error")
				assert.Equal(t, tt.expectedArgs, capturedArgs, tt.description)
			} else {
				// For negative test cases, we might expect an error or no call
				assert.Nil(t, capturedArgs, "Handler should not be called for invalid commands")
			}
		})
	}
}

func TestBasicCommandStillWorks(t *testing.T) {
	handler := createTestHandler()

	// Mock command function for testing
	var capturedArgs []string
	var commandCalled bool
	mockYankHandler := func(args []string) error {
		commandCalled = true
		capturedArgs = args
		return nil
	}

	// Register a yank command with alias
	yankCmd := &mockCommand{
		BaseCommand: BaseCommand{
			Name:        "yank",
			Description: "Copy messages to clipboard",
			Usage:       ":y[count][direction]",
			Aliases:     []string{"y"},
		},
		executeFunc: mockYankHandler,
	}
	handler.RegisterNewCommand(yankCmd)

	t.Run("basic y command should work", func(t *testing.T) {
		// Reset
		capturedArgs = nil
		commandCalled = false

		// Execute basic :y command
		err := handler.HandleCommand(":y", []string{})

		assert.NoError(t, err, "Basic :y command should execute without error")
		assert.True(t, commandCalled, "Handler should be called for :y")
		assert.Equal(t, []string{}, capturedArgs, "Basic :y should have empty args")
	})

	t.Run("yank command should work", func(t *testing.T) {
		// Reset
		capturedArgs = nil
		commandCalled = false

		// Execute :yank command
		err := handler.HandleCommand(":yank", []string{})

		assert.NoError(t, err, "Basic :yank command should execute without error")
		assert.True(t, commandCalled, "Handler should be called for :yank")
		assert.Equal(t, []string{}, capturedArgs, "Basic :yank should have empty args")
	})

	t.Run("y with args should work", func(t *testing.T) {
		// Reset
		capturedArgs = nil
		commandCalled = false

		// Execute :y with args
		err := handler.HandleCommand(":y", []string{"2k"})

		assert.NoError(t, err, ":y with args should execute without error")
		assert.True(t, commandCalled, "Handler should be called for :y with args")
		assert.Equal(t, []string{"2k"}, capturedArgs, ":y should pass through args")
	})
}

func TestRealWorldScenario(t *testing.T) {
	// Test the exact scenario the user is experiencing
	handler := createTestHandler()

	var yankCalled bool
	var yankArgs []string

	// Register yank command exactly like in the real app
	yankCmd := &mockCommand{
		BaseCommand: BaseCommand{
			Name:        "yank",
			Description: "Copy messages to clipboard (vim-style)",
			Usage:       ":y[count][direction]",
			Examples: []string{
				":y",
				":y3",
				":y2k",
				":y5j",
			},
			Aliases:  []string{"y"},
			Category: "Clipboard",
		},
		executeFunc: func(args []string) error {
			yankCalled = true
			yankArgs = args
			return nil
		},
	}
	handler.RegisterNewCommand(yankCmd)

	t.Run("y should work like yank", func(t *testing.T) {
		// Reset
		yankCalled = false
		yankArgs = nil

		// This should work
		err := handler.HandleCommand(":y", []string{})
		assert.NoError(t, err)
		assert.True(t, yankCalled, ":y should call yank handler")
		assert.Equal(t, []string{}, yankArgs)
	})

	t.Run("yank should work", func(t *testing.T) {
		// Reset
		yankCalled = false
		yankArgs = nil

		// This should also work
		err := handler.HandleCommand(":yank", []string{})
		assert.NoError(t, err)
		assert.True(t, yankCalled, ":yank should call yank handler")
		assert.Equal(t, []string{}, yankArgs)
	})

	t.Run("y1k should work as vim-style", func(t *testing.T) {
		// Reset
		yankCalled = false
		yankArgs = nil

		// This should work via vim-style parsing
		err := handler.HandleCommand(":y1k", []string{})
		assert.NoError(t, err)
		assert.True(t, yankCalled, ":y1k should call yank handler")
		assert.Equal(t, []string{"1k"}, yankArgs, ":y1k should parse as y with arg 1k")
	})
}

func TestStringHandling(t *testing.T) {
	// Test the exact scenario the user is experiencing
	handler := createTestHandler()

	var capturedCommand string
	mockHandler := func(args []string) error {
		capturedCommand = "called"
		return nil
	}

	yankCmd := &mockCommand{
		BaseCommand: BaseCommand{
			Name:    "yank",
			Aliases: []string{"y"},
		},
		executeFunc: mockHandler,
	}
	handler.RegisterNewCommand(yankCmd)

	t.Run("exact string handling", func(t *testing.T) {
		capturedCommand = ""

		// Test exact strings
		testCases := []string{
			":y",
			": y", // with space
			":y ", // trailing space
		}

		for _, testCase := range testCases {
			capturedCommand = ""
			err := handler.HandleCommand(testCase, []string{})
			assert.NoError(t, err, "Should handle %q", testCase)
			if testCase == ":y" {
				assert.Equal(t, "called", capturedCommand, "Should call handler for %q", testCase)
			}
		}
	})

	t.Run("trim prefix behavior", func(t *testing.T) {
		// Test the exact trimming behavior
		assert.Equal(t, "y", strings.TrimPrefix(":y", ":"))
		assert.Equal(t, " y", strings.TrimPrefix(": y", ":")) // space remains
		assert.Equal(t, "y ", strings.TrimPrefix(":y ", ":")) // trailing space remains
	})
}

func TestVimStyleParsingEdgeCases(t *testing.T) {
	notification := &types.MockNotification{}

	// Test the exact scenario the user is experiencing
	eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus, notification)

	// Register a yank command
	mockHandler := func(args []string) error { return nil }
	yankCmd := &mockCommand{
		BaseCommand: BaseCommand{
			Name:    "yank",
			Aliases: []string{"y"},
		},
		executeFunc: mockHandler,
	}
	handler.RegisterNewCommand(yankCmd)

	t.Run("non-vim command not affected", func(t *testing.T) {
		// This should trigger unknown command, not vim parsing
		err := handler.HandleCommand(":config1", []string{})
		assert.NoError(t, err) // No error because we handle unknown commands gracefully
		if len(notification.SystemMessages) != 1 {
			t.Errorf("expected 1 system message, got %d", len(notification.SystemMessages))
		}
	})

	t.Run("vim command with no suffix", func(t *testing.T) {
		// :y should work normally (exact match)
		err := handler.HandleCommand(":y", []string{})
		assert.NoError(t, err)
	})

	t.Run("vim command alias matching", func(t *testing.T) {
		// Both :y123 and :yank123 should work
		var capturedArgs []string

		yankCmd := &mockCommand{
			BaseCommand: BaseCommand{
				Name:    "yank",
				Aliases: []string{"y"},
			},
			executeFunc: func(args []string) error {
				capturedArgs = args
				return nil
			},
		}

		handler := createTestHandler()
		handler.RegisterNewCommand(yankCmd)

		// Test alias parsing
		err := handler.HandleCommand(":y123", []string{})
		assert.NoError(t, err)
		assert.Equal(t, []string{"123"}, capturedArgs)

		// Test primary name parsing
		capturedArgs = nil
		err = handler.HandleCommand(":yank456", []string{})
		assert.NoError(t, err)
		assert.Equal(t, []string{"456"}, capturedArgs)
	})
}

