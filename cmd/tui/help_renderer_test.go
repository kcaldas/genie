package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kcaldas/genie/cmd/slashcommands"
	"github.com/kcaldas/genie/cmd/tui/controllers/commands"
	"github.com/stretchr/testify/assert"
)

func TestManPageHelpRenderer_RenderSlashCommands(t *testing.T) {
	// Create temporary directory for test slash commands
	tempDir, err := os.MkdirTemp("", "genie_slash_help_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test command files
	projectDir := filepath.Join(tempDir, ".genie", "commands")
	userDir := filepath.Join(tempDir, "user", ".genie", "commands")
	os.MkdirAll(projectDir, 0755)
	os.MkdirAll(userDir, 0755)

	// Create project commands
	os.WriteFile(filepath.Join(projectDir, "compact.md"), []byte("Please provide a compact summary of the code or information provided.\n\nFocus on key points.\n\n$ARGUMENTS"), 0644)
	os.WriteFile(filepath.Join(projectDir, "verbose.md"), []byte("Provide detailed explanation with examples and documentation"), 0644)

	// Create user commands
	os.WriteFile(filepath.Join(userDir, "custom.md"), []byte("My custom command"), 0644)

	// Set up slash command manager
	manager := slashcommands.NewManager()
	err = manager.DiscoverCommands(tempDir, func() (string, error) {
		return filepath.Join(tempDir, "user"), nil
	})
	assert.NoError(t, err)

	// Create minimal registry and keymap for testing
	registry := commands.NewCommandRegistry()
	keymap := NewKeymap()

	// Create help renderer
	renderer := NewManPageHelpRenderer(registry, keymap, manager)

	// Test RenderSlashCommands
	helpText := renderer.RenderSlashCommands()

	// Verify basic structure
	assert.Contains(t, helpText, "# SLASH COMMANDS")
	
	// Verify quick reference list comes first
	assert.Contains(t, helpText, "**Available Commands:**")
	assert.Contains(t, helpText, "- `/compact`")
	assert.Contains(t, helpText, "- `/verbose`")
	assert.Contains(t, helpText, "- `/custom`")
	
	// Verify that Available Commands appears before Project Commands
	availableIdx := strings.Index(helpText, "**Available Commands:**")
	projectIdx := strings.Index(helpText, "## Project Commands")
	assert.True(t, availableIdx < projectIdx, "Available Commands should appear before Project Commands")

	// Verify project commands section
	assert.Contains(t, helpText, "## Project Commands")
	assert.Contains(t, helpText, "### /compact")
	assert.Contains(t, helpText, "### /verbose")
	assert.Contains(t, helpText, "**Please provide a compact summary of the code or information provided.**")
	assert.Contains(t, helpText, "```")
	assert.Contains(t, helpText, "Focus on key points")

	// Verify user commands section
	assert.Contains(t, helpText, "## User Commands")
	assert.Contains(t, helpText, "### /custom")
	assert.Contains(t, helpText, "**My custom command**")

	// Verify usage information
	assert.Contains(t, helpText, "## USAGE")
	assert.Contains(t, helpText, "## COMMAND LOCATIONS")

	// Verify source information is displayed
	assert.Contains(t, helpText, "*Source: project*")
	assert.Contains(t, helpText, "*Source: user*")
}

func TestManPageHelpRenderer_RenderSlashCommands_NoCommands(t *testing.T) {
	// Empty slash command manager
	manager := slashcommands.NewManager()
	registry := commands.NewCommandRegistry()
	keymap := NewKeymap()

	renderer := NewManPageHelpRenderer(registry, keymap, manager)
	helpText := renderer.RenderSlashCommands()

	// Should show "no commands found" message
	assert.Contains(t, helpText, "**No slash commands found.**")
	assert.Contains(t, helpText, "Slash commands can be defined as markdown files in:")
	assert.Contains(t, helpText, ".genie/commands/")
	assert.Contains(t, helpText, ".claude/commands/")
}

func TestManPageHelpRenderer_FormatSlashCommand(t *testing.T) {
	manager := slashcommands.NewManager()
	registry := commands.NewCommandRegistry()
	keymap := NewKeymap()
	renderer := NewManPageHelpRenderer(registry, keymap, manager).(*ManPageHelpRenderer)

	// Test basic command
	cmd := slashcommands.SlashCommand{
		Name:        "test",
		Description: "A test command for formatting\n\nWith multiple lines\n$ARGUMENTS",
		Source:      "project",
	}

	formatted := renderer.formatSlashCommand("test", cmd)

	assert.Contains(t, formatted, "### /test")
	assert.Contains(t, formatted, "**A test command for formatting**")
	assert.Contains(t, formatted, "```")
	assert.Contains(t, formatted, "With multiple lines")
	assert.Contains(t, formatted, "*Source: project*")

	// Test long description truncation
	longDesc := "This is a very long description that should be truncated. " +
		"It contains a lot of text to test the truncation functionality. " +
		"The truncation should happen at a word boundary to ensure readability. " +
		"This text is definitely longer than 200 characters and will be cut off."
	
	cmdLong := slashcommands.SlashCommand{
		Name:        "longtest",
		Description: longDesc,
		Source:      "user",
	}

	formattedLong := renderer.formatSlashCommand("longtest", cmdLong)
	assert.Contains(t, formattedLong, "### /longtest")
	assert.Contains(t, formattedLong, "...")
	assert.Contains(t, formattedLong, "```")
}