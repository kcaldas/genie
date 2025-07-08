package controllers

import (
	"sort"
	"testing"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCommandFunc provides a simple handler for testing
func mockCommandFunc(args []string) error {
	return nil
}

func TestCommandRegistry(t *testing.T) {
	t.Run("new registry initialization", func(t *testing.T) {
		registry := NewCommandRegistry()

		assert.NotNil(t, registry)
		assert.NotNil(t, registry.commands)
		assert.NotNil(t, registry.aliasToCommand)
		assert.NotNil(t, registry.categories)
		assert.Empty(t, registry.GetAllCommands())
	})

	t.Run("register single command", func(t *testing.T) {
		registry := NewCommandRegistry()

		cmd := &Command{
			Name:        "test",
			Description: "A test command",
			Usage:       ":test",
			Examples:    []string{":test"},
			Aliases:     []string{"t"},
			Category:    "Testing",
			Handler:     mockCommandFunc,
		}

		registry.Register(cmd)

		// Test primary name lookup
		retrieved := registry.GetCommand("test")
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test", retrieved.GetName())
		assert.Equal(t, "A test command", retrieved.GetDescription())

		// Test alias lookup
		retrievedByAlias := registry.GetCommand("t")
		assert.NotNil(t, retrievedByAlias)
		assert.Equal(t, retrieved, retrievedByAlias) // Compare wrappers, not wrapper to original

		// Test category registration
		categories := registry.GetCommandsByCategory()
		assert.Contains(t, categories, "Testing")
		assert.Len(t, categories["Testing"], 1)
		assert.Equal(t, retrieved, categories["Testing"][0]) // Compare with retrieved wrapper
	})

	t.Run("register command with multiple aliases", func(t *testing.T) {
		registry := NewCommandRegistry()

		cmd := &Command{
			Name:        "help",
			Description: "Show help information",
			Usage:       ":help [command]",
			Examples:    []string{":help", ":help config"},
			Aliases:     []string{"h", "?", "man"},
			Category:    "General",
			Handler:     mockCommandFunc,
		}

		registry.Register(cmd)

		// Get the wrapper first
		helpWrapper := registry.GetCommand("help")
		assert.NotNil(t, helpWrapper)

		// Test all aliases work
		for _, alias := range cmd.Aliases {
			retrieved := registry.GetCommand(alias)
			assert.NotNil(t, retrieved, "Alias %s should work", alias)
			assert.Equal(t, helpWrapper, retrieved, "Alias %s should return the same command", alias)
		}

		// Test primary name still works
		retrieved := registry.GetCommand("help")
		assert.Equal(t, helpWrapper, retrieved)
	})

	t.Run("register multiple commands in same category", func(t *testing.T) {
		registry := NewCommandRegistry()

		cmd1 := &Command{
			Name:        "config",
			Description: "Configure settings",
			Category:    "Configuration",
			Handler:     mockCommandFunc,
		}

		cmd2 := &Command{
			Name:        "theme",
			Description: "Change theme",
			Category:    "Configuration",
			Handler:     mockCommandFunc,
		}

		registry.Register(cmd1)
		registry.Register(cmd2)

		categories := registry.GetCommandsByCategory()
		assert.Contains(t, categories, "Configuration")
		assert.Len(t, categories["Configuration"], 2)

		// Commands should be sorted by name
		configCommands := categories["Configuration"]
		assert.Equal(t, "config", configCommands[0].GetName())
		assert.Equal(t, "theme", configCommands[1].GetName())
	})

	t.Run("hidden commands", func(t *testing.T) {
		registry := NewCommandRegistry()

		visibleCmd := &Command{
			Name:        "visible",
			Description: "A visible command",
			Category:    "General",
			Handler:     mockCommandFunc,
			Hidden:      false,
		}

		hiddenCmd := &Command{
			Name:        "hidden",
			Description: "A hidden command",
			Category:    "General",
			Handler:     mockCommandFunc,
			Hidden:      true,
		}

		registry.Register(visibleCmd)
		registry.Register(hiddenCmd)

		// GetAllCommands should only return visible commands
		allCommands := registry.GetAllCommands()
		assert.Len(t, allCommands, 1)
		assert.Equal(t, "visible", allCommands[0].GetName())

		// GetCommandsByCategory should only return visible commands
		categories := registry.GetCommandsByCategory()
		assert.Len(t, categories["General"], 1)
		assert.Equal(t, "visible", categories["General"][0].GetName())

		// GetCommandNames should only return visible command names
		names := registry.GetCommandNames()
		assert.Contains(t, names, "visible")
		assert.NotContains(t, names, "hidden")

		// But GetCommand should still find hidden commands
		retrievedHidden := registry.GetCommand("hidden")
		assert.NotNil(t, retrievedHidden)
		assert.Equal(t, "hidden", retrievedHidden.GetName())
	})

	t.Run("command with no category", func(t *testing.T) {
		registry := NewCommandRegistry()

		cmd := &Command{
			Name:        "nocategory",
			Description: "Command without category",
			Category:    "", // Empty category
			Handler:     mockCommandFunc,
		}

		registry.Register(cmd)

		// Should still be retrievable by name
		retrieved := registry.GetCommand("nocategory")
		assert.NotNil(t, retrieved)
		assert.Equal(t, "nocategory", retrieved.GetName())

		// Should appear in all commands
		allCommands := registry.GetAllCommands()
		assert.Len(t, allCommands, 1)
		assert.Equal(t, "nocategory", allCommands[0].GetName())

		// Should not appear in any category
		categories := registry.GetCommandsByCategory()
		assert.Empty(t, categories)
	})

	t.Run("search commands", func(t *testing.T) {
		registry := NewCommandRegistry()

		commands := []*Command{
			{
				Name:        "config",
				Description: "Configure application settings",
				Aliases:     []string{"cfg", "settings"},
				Category:    "Configuration",
				Handler:     mockCommandFunc,
			},
			{
				Name:        "help",
				Description: "Show help information",
				Aliases:     []string{"h"},
				Category:    "General",
				Handler:     mockCommandFunc,
			},
			{
				Name:        "theme",
				Description: "Change color theme",
				Category:    "Configuration",
				Handler:     mockCommandFunc,
			},
			{
				Name:        "hidden",
				Description: "Hidden command",
				Category:    "Internal",
				Handler:     mockCommandFunc,
				Hidden:      true,
			},
		}

		for _, cmd := range commands {
			registry.Register(cmd)
		}

		// Search by name
		results := registry.SearchCommands("config")
		assert.Len(t, results, 1)
		assert.Equal(t, "config", results[0].GetName())

		// Search by alias
		results = registry.SearchCommands("cfg")
		assert.Len(t, results, 1)
		assert.Equal(t, "config", results[0].GetName())

		// Search by description
		results = registry.SearchCommands("help")
		assert.Len(t, results, 1)
		assert.Equal(t, "help", results[0].GetName())

		// Search by partial match
		results = registry.SearchCommands("the")
		assert.Len(t, results, 1)
		assert.Equal(t, "theme", results[0].GetName())

		// Search should not return hidden commands
		results = registry.SearchCommands("hidden")
		assert.Empty(t, results)

		// Search with no matches
		results = registry.SearchCommands("nonexistent")
		assert.Empty(t, results)

		// Case insensitive search
		results = registry.SearchCommands("CONFIG")
		assert.Len(t, results, 1)
		assert.Equal(t, "config", results[0].GetName())
	})

	t.Run("get aliases", func(t *testing.T) {
		registry := NewCommandRegistry()

		cmd := &Command{
			Name:        "exit",
			Description: "Exit application",
			Aliases:     []string{"quit", "q", "bye"},
			Category:    "General",
			Handler:     mockCommandFunc,
		}

		registry.Register(cmd)

		aliases := registry.GetAliases("exit")
		assert.Len(t, aliases, 3)
		assert.Contains(t, aliases, "quit")
		assert.Contains(t, aliases, "q")
		assert.Contains(t, aliases, "bye")

		// Non-existent command should return nil
		aliases = registry.GetAliases("nonexistent")
		assert.Nil(t, aliases)
	})

	t.Run("get categories", func(t *testing.T) {
		registry := NewCommandRegistry()

		commands := []*Command{
			{Name: "cmd1", Category: "General", Handler: mockCommandFunc},
			{Name: "cmd2", Category: "Configuration", Handler: mockCommandFunc},
			{Name: "cmd3", Category: "Debug", Handler: mockCommandFunc},
			{Name: "cmd4", Category: "General", Handler: mockCommandFunc},
		}

		for _, cmd := range commands {
			registry.Register(cmd)
		}

		categories := registry.GetCategories()
		assert.Len(t, categories, 3)

		// Should be sorted
		expectedCategories := []string{"Configuration", "Debug", "General"}
		sort.Strings(expectedCategories)
		assert.Equal(t, expectedCategories, categories)
	})

	t.Run("get command names", func(t *testing.T) {
		registry := NewCommandRegistry()

		cmd1 := &Command{
			Name:     "help",
			Aliases:  []string{"h", "?"},
			Category: "General",
			Handler:  mockCommandFunc,
		}

		cmd2 := &Command{
			Name:     "exit",
			Aliases:  []string{"quit", "q"},
			Category: "General",
			Handler:  mockCommandFunc,
		}

		hiddenCmd := &Command{
			Name:     "internal",
			Aliases:  []string{"int"},
			Category: "Internal",
			Handler:  mockCommandFunc,
			Hidden:   true,
		}

		registry.Register(cmd1)
		registry.Register(cmd2)
		registry.Register(hiddenCmd)

		names := registry.GetCommandNames()

		// Should include primary names and aliases for visible commands
		expectedNames := []string{"help", "h", "?", "exit", "quit", "q"}
		sort.Strings(expectedNames)

		// Should be sorted
		sort.Strings(names)
		assert.Equal(t, expectedNames, names)

		// Should not include hidden command names or aliases
		assert.NotContains(t, names, "internal")
		assert.NotContains(t, names, "int")
	})

	t.Run("command sorting", func(t *testing.T) {
		registry := NewCommandRegistry()

		// Register commands in random order
		commands := []*Command{
			{Name: "zebra", Category: "Animals", Handler: mockCommandFunc},
			{Name: "apple", Category: "Fruits", Handler: mockCommandFunc},
			{Name: "banana", Category: "Fruits", Handler: mockCommandFunc},
			{Name: "cat", Category: "Animals", Handler: mockCommandFunc},
		}

		for _, cmd := range commands {
			registry.Register(cmd)
		}

		// GetAllCommands should be sorted by name
		allCommands := registry.GetAllCommands()
		assert.Equal(t, "apple", allCommands[0].GetName())
		assert.Equal(t, "banana", allCommands[1].GetName())
		assert.Equal(t, "cat", allCommands[2].GetName())
		assert.Equal(t, "zebra", allCommands[3].GetName())

		// Commands within categories should be sorted
		categories := registry.GetCommandsByCategory()

		fruitsCommands := categories["Fruits"]
		assert.Equal(t, "apple", fruitsCommands[0].GetName())
		assert.Equal(t, "banana", fruitsCommands[1].GetName())

		animalCommands := categories["Animals"]
		assert.Equal(t, "cat", animalCommands[0].GetName())
		assert.Equal(t, "zebra", animalCommands[1].GetName())
	})

	t.Run("edge cases", func(t *testing.T) {
		registry := NewCommandRegistry()

		// Try to get non-existent command
		cmd := registry.GetCommand("nonexistent")
		assert.Nil(t, cmd)

		// Empty registry operations
		assert.Empty(t, registry.GetAllCommands())
		assert.Empty(t, registry.GetCommandsByCategory())
		assert.Empty(t, registry.GetCommandNames())
		assert.Empty(t, registry.GetCategories())
		assert.Empty(t, registry.SearchCommands("anything"))

		// Register command with empty name (edge case)
		emptyCmd := &Command{
			Name:        "",
			Description: "Empty name command",
			Handler:     mockCommandFunc,
		}
		registry.Register(emptyCmd)

		// Should still be registered (though not recommended)
		retrieved := registry.GetCommand("")
		assert.NotNil(t, retrieved)
		assert.Equal(t, "", retrieved.GetName())
	})
}

func TestCommandHandlerWithRegistry(t *testing.T) {
	t.Run("new handler initialization", func(t *testing.T) {
		eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus)

		assert.NotNil(t, handler)
		assert.NotNil(t, handler.registry)
		assert.NotNil(t, handler.commands)
		assert.NotNil(t, handler.aliases)
		assert.Empty(t, handler.GetAvailableCommands())
	})

	t.Run("register command with metadata", func(t *testing.T) {
		eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus)

		cmd := &Command{
			Name:        "test",
			Description: "Test command",
			Usage:       ":test <arg>",
			Examples:    []string{":test hello", ":test world"},
			Aliases:     []string{"t"},
			Category:    "Testing",
			Handler:     mockCommandFunc,
		}

		handler.RegisterCommandWithMetadata(cmd)

		// Should be retrievable via registry
		retrieved := handler.GetCommand("test")
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test", retrieved.GetName())

		// Should appear in available commands
		available := handler.GetAvailableCommands()
		assert.Contains(t, available, "test")

		// Should work with HandleCommand
		err := handler.HandleCommand(":test", []string{"arg"})
		assert.NoError(t, err)

		// Should work with alias
		err = handler.HandleCommand(":t", []string{"arg"})
		assert.NoError(t, err)
	})

	t.Run("backward compatibility with legacy registration", func(t *testing.T) {
		eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus)

		// Register using legacy method
		handler.RegisterCommand("legacy", mockCommandFunc)
		handler.RegisterAlias("l", "legacy")

		// Should work with HandleCommand
		err := handler.HandleCommand(":legacy", []string{})
		assert.NoError(t, err)

		err = handler.HandleCommand(":l", []string{})
		assert.NoError(t, err)

		// Should appear in available commands
		available := handler.GetAvailableCommands()
		assert.Contains(t, available, "legacy")
	})

	t.Run("mixed registration (registry + legacy)", func(t *testing.T) {
		eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus)

		// Register with metadata
		metadataCmd := &Command{
			Name:        "modern",
			Description: "Modern command",
			Aliases:     []string{"m"},
			Handler:     mockCommandFunc,
		}
		handler.RegisterCommandWithMetadata(metadataCmd)

		// Register legacy
		handler.RegisterCommand("legacy", mockCommandFunc)
		handler.RegisterAlias("l", "legacy")

		// Both should work
		err := handler.HandleCommand(":modern", []string{})
		assert.NoError(t, err)

		err = handler.HandleCommand(":m", []string{})
		assert.NoError(t, err)

		err = handler.HandleCommand(":legacy", []string{})
		assert.NoError(t, err)

		err = handler.HandleCommand(":l", []string{})
		assert.NoError(t, err)

		// Both should appear in available commands
		available := handler.GetAvailableCommands()
		assert.Contains(t, available, "modern")
		assert.Contains(t, available, "legacy")
	})

	t.Run("unknown command handling", func(t *testing.T) {
		eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus)

		// Unknown commands should not return an error (graceful handling)
		err := handler.HandleCommand(":unknown", []string{})
		assert.NoError(t, err, "Unknown commands should be handled gracefully")

		// Test with unknown command callback
		var calledWith string
		handler.SetUnknownCommandHandler(func(cmd string) {
			calledWith = cmd
		})

		err = handler.HandleCommand(":nonexistent", []string{})
		assert.NoError(t, err)
		assert.Equal(t, "nonexistent", calledWith)
	})

	t.Run("command with slash prefix handling", func(t *testing.T) {
		eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus)

		cmd := &Command{
			Name:        "test",
			Description: "Test command",
			Handler:     mockCommandFunc,
		}

		handler.RegisterCommandWithMetadata(cmd)

		// Should work both with and without slash prefix
		err := handler.HandleCommand("test", []string{})
		assert.NoError(t, err)

		err = handler.HandleCommand(":test", []string{})
		assert.NoError(t, err)
	})

	t.Run("get registry", func(t *testing.T) {
		eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus)

		registry := handler.GetRegistry()
		assert.NotNil(t, registry)
		assert.IsType(t, &CommandRegistry{}, registry)
	})
}

func TestCommandStructure(t *testing.T) {
	t.Run("command with all fields", func(t *testing.T) {
		cmd := &Command{
			Name:        "complete",
			Description: "A complete command example",
			Usage:       ":complete <required> [optional]",
			Examples: []string{
				":complete hello",
				":complete hello world",
			},
			Aliases:  []string{"comp", "c"},
			Category: "Examples",
			Handler:  mockCommandFunc,
			Hidden:   false,
		}

		assert.Equal(t, "complete", cmd.Name)
		assert.Equal(t, "A complete command example", cmd.Description)
		assert.Equal(t, ":complete <required> [optional]", cmd.Usage)
		assert.Len(t, cmd.Examples, 2)
		assert.Len(t, cmd.Aliases, 2)
		assert.Equal(t, "Examples", cmd.Category)
		assert.NotNil(t, cmd.Handler)
		assert.False(t, cmd.Hidden)
	})

	t.Run("minimal command", func(t *testing.T) {
		cmd := &Command{
			Name:    "minimal",
			Handler: mockCommandFunc,
		}

		assert.Equal(t, "minimal", cmd.Name)
		assert.Equal(t, "", cmd.Description)
		assert.Equal(t, "", cmd.Usage)
		assert.Empty(t, cmd.Examples)
		assert.Empty(t, cmd.Aliases)
		assert.Equal(t, "", cmd.Category)
		assert.NotNil(t, cmd.Handler)
		assert.False(t, cmd.Hidden)
	})
}

// Integration test that mimics real usage
func TestCommandRegistryIntegration(t *testing.T) {
	t.Run("realistic command setup", func(t *testing.T) {
		eventBus := events.NewCommandEventBus()
	handler := NewCommandHandler(eventBus)

		// Register commands similar to the actual app
		commands := []*Command{
			{
				Name:        "help",
				Description: "Show help message with available commands and shortcuts",
				Usage:       ":help [command]",
				Examples:    []string{":help", ":help config", ":help theme"},
				Aliases:     []string{"h"},
				Category:    "General",
				Handler:     mockCommandFunc,
			},
			{
				Name:        "clear",
				Description: "Clear the conversation history",
				Usage:       ":clear",
				Examples:    []string{":clear"},
				Aliases:     []string{"cls"},
				Category:    "Chat",
				Handler:     mockCommandFunc,
			},
			{
				Name:        "config",
				Description: "Open configuration menu or set specific configuration values",
				Usage:       ":config [setting] [value]",
				Examples:    []string{":config", ":config theme dark", ":config cursor true"},
				Aliases:     []string{"cfg", "settings"},
				Category:    "Configuration",
				Handler:     mockCommandFunc,
			},
			{
				Name:        "debug",
				Description: "Toggle debug panel visibility to show tool calls and system events",
				Usage:       ":debug",
				Examples:    []string{":debug"},
				Category:    "Debug",
				Handler:     mockCommandFunc,
			},
			{
				Name:        "exit",
				Description: "Exit the application",
				Usage:       ":exit",
				Examples:    []string{":exit"},
				Aliases:     []string{"quit", "q"},
				Category:    "General",
				Handler:     mockCommandFunc,
			},
		}

		// Register all commands
		for _, cmd := range commands {
			handler.RegisterCommandWithMetadata(cmd)
		}

		// Test command retrieval
		helpCmd := handler.GetCommand("help")
		require.NotNil(t, helpCmd)
		assert.Equal(t, "help", helpCmd.GetName())
		assert.Contains(t, helpCmd.GetAliases(), "h")

		// Test alias resolution
		helpByAlias := handler.GetCommand("h")
		assert.Equal(t, helpCmd, helpByAlias)

		// Test category grouping
		registry := handler.GetRegistry()
		categories := registry.GetCommandsByCategory()

		assert.Contains(t, categories, "General")
		assert.Contains(t, categories, "Chat")
		assert.Contains(t, categories, "Configuration")
		assert.Contains(t, categories, "Debug")

		// Test that General category has both help and exit
		generalCommands := categories["General"]
		assert.Len(t, generalCommands, 2)

		// Commands should be sorted
		assert.Equal(t, "exit", generalCommands[0].GetName())
		assert.Equal(t, "help", generalCommands[1].GetName())

		// Test command execution
		for _, cmd := range commands {
			err := handler.HandleCommand(":"+cmd.Name, []string{})
			assert.NoError(t, err, "Command %s should execute without error", cmd.Name)

			// Test aliases
			for _, alias := range cmd.Aliases {
				err := handler.HandleCommand(":"+alias, []string{})
				assert.NoError(t, err, "Alias %s should execute without error", alias)
			}
		}

		// Test search functionality
		configResults := registry.SearchCommands("config")
		assert.Len(t, configResults, 1)
		assert.Equal(t, "config", configResults[0].GetName())

		settingsResults := registry.SearchCommands("settings")
		assert.Len(t, settingsResults, 1)
		assert.Equal(t, "config", settingsResults[0].GetName())
	})
}

