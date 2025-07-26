package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/slashcommands"
	"github.com/kcaldas/genie/cmd/tui/controllers/commands"
)

// HelpRenderer generates man-style help documentation from CommandRegistry and Keymap
type HelpRenderer interface {
	// RenderHelp generates complete help text in man-page style
	RenderHelp() string
	// RenderCommands generates the COMMANDS section
	RenderCommands() string
	// RenderShortcuts generates the SHORTCUTS section
	RenderShortcuts() string
	// RenderCommandsByCategory generates commands grouped by category
	RenderCommandsByCategory() string
	// RenderSlashCommands generates slash commands help
	RenderSlashCommands() string
}

// ManPageHelpRenderer implements HelpRenderer with man-page style formatting
type ManPageHelpRenderer struct {
	registry           *commands.CommandRegistry
	keymap             *Keymap
	slashCommandManager *slashcommands.Manager
}

// NewManPageHelpRenderer creates a new help renderer
func NewManPageHelpRenderer(registry *commands.CommandRegistry, keymap *Keymap, slashCommandManager *slashcommands.Manager) HelpRenderer {
	return &ManPageHelpRenderer{
		registry:           registry,
		keymap:             keymap,
		slashCommandManager: slashCommandManager,
	}
}

// RenderHelp generates complete man-style help documentation
func (h *ManPageHelpRenderer) RenderHelp() string {
	var sb strings.Builder

	// Header
	sb.WriteString("\n# GENIE(1)\n\n")
	sb.WriteString("## NAME\n")
	sb.WriteString("genie - AI-powered coding assistant with interactive TUI\n\n")

	sb.WriteString("## DESCRIPTION\n")
	sb.WriteString("Genie is an interactive terminal user interface for AI-assisted software development.\n")
	sb.WriteString("It supports both vi-style text commands and direct keyboard shortcuts.\n\n")

	// Shortcuts section (moved before commands)
	sb.WriteString("## SHORTCUTS\n")
	sb.WriteString("Keyboard shortcuts work globally and provide quick access to common actions.\n\n")
	sb.WriteString(h.RenderShortcuts())

	// Commands section
	sb.WriteString("\n## COMMANDS\n")
	sb.WriteString("Commands are entered by typing `:` followed by the command name in the input field.\n\n")
	sb.WriteString(h.RenderCommandsByCategory())

	// Examples section
	sb.WriteString("\n## EXAMPLES\n")
	sb.WriteString(h.generateExamples())

	// Footer
	sb.WriteString("\n## SEE ALSO\n")
	sb.WriteString("For more information, visit the Genie documentation.\n")

	return sb.String()
}

// RenderCommands generates the COMMANDS section (flat list)
func (h *ManPageHelpRenderer) RenderCommands() string {
	var sb strings.Builder

	commands := h.registry.GetAllCommands()

	// Sort commands alphabetically
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].GetName() < commands[j].GetName()
	})

	for _, cmd := range commands {
		sb.WriteString(h.formatCommand(cmd))
	}

	return sb.String()
}

// RenderCommandsByCategory generates commands grouped by category
func (h *ManPageHelpRenderer) RenderCommandsByCategory() string {
	var sb strings.Builder

	commandsByCategory := h.registry.GetCommandsByCategory()

	// Get categories and sort them
	var categories []string
	for category := range commandsByCategory {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	for _, category := range categories {
		commands := commandsByCategory[category]
		if len(commands) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s\n", category))

		// Sort commands within category
		sort.Slice(commands, func(i, j int) bool {
			return commands[i].GetName() < commands[j].GetName()
		})

		for _, cmd := range commands {
			sb.WriteString(h.formatCommand(cmd))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderShortcuts generates the SHORTCUTS section by dynamically analyzing keymap
func (h *ManPageHelpRenderer) RenderShortcuts() string {
	var sb strings.Builder

	entries := h.keymap.GetEntries()

	// Collect all shortcuts in a single list for easy scanning
	allShortcuts := make([]KeymapEntry, 0)
	for _, entry := range entries {
		allShortcuts = append(allShortcuts, entry)
	}

	// Sort shortcuts by key name for consistent output
	sort.Slice(allShortcuts, func(i, j int) bool {
		return h.formatKeyName(allShortcuts[i].Key, allShortcuts[i].Mod) <
			h.formatKeyName(allShortcuts[j].Key, allShortcuts[j].Mod)
	})

	// Simple bullet list format for easy scanning
	for _, entry := range allShortcuts {
		keyName := h.formatKeyName(entry.Key, entry.Mod)
		var description string

		if entry.Action.Type == "command" {
			// For command shortcuts, show command equivalent
			cmd := h.registry.GetCommand(entry.Action.CommandName)
			if cmd != nil {
				description = fmt.Sprintf("%s (same as `:%s`)", cmd.GetDescription(), entry.Action.CommandName)
			} else {
				description = fmt.Sprintf("%s (same as `:%s`)", entry.Description, entry.Action.CommandName)
			}
		} else {
			// For direct action shortcuts, just show description
			description = entry.Description
		}

		sb.WriteString(fmt.Sprintf("- `%s` - %s\n", keyName, description))
	}

	return sb.String()
}

// formatCommand formats a single command in man-page style
func (h *ManPageHelpRenderer) formatCommand(cmd *commands.CommandWrapper) string {
	var sb strings.Builder

	// Command name with aliases
	sb.WriteString(fmt.Sprintf("**:%s**", cmd.GetName()))
	if len(cmd.GetAliases()) > 0 {
		aliasStr := strings.Join(cmd.GetAliases(), ", :")
		sb.WriteString(fmt.Sprintf(" (:%s)", aliasStr))
	}

	// Add shortcut if this command has one
	if shortcut := h.findShortcutForCommand(cmd.GetName()); shortcut != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", shortcut))
	}

	sb.WriteString("  \n")

	// Description
	if cmd.GetDescription() != "" {
		sb.WriteString(fmt.Sprintf("    %s  \n", cmd.GetDescription()))
	}

	// Usage
	if cmd.GetUsage() != "" {
		sb.WriteString(fmt.Sprintf("\nUsage: `%s`  \n", cmd.GetUsage()))
	}

	// Examples
	if len(cmd.GetExamples()) > 0 {
		sb.WriteString("\nExamples:  \n  \n")
		for _, example := range cmd.GetExamples() {
			sb.WriteString(fmt.Sprintf("- `%s`  \n", example))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// findShortcutForCommand dynamically finds shortcuts that map to a command
func (h *ManPageHelpRenderer) findShortcutForCommand(commandName string) string {
	entries := h.keymap.GetEntries()

	for _, entry := range entries {
		if entry.Action.Type == "command" && entry.Action.CommandName == commandName {
			return h.formatKeyName(entry.Key, entry.Mod)
		}
	}

	return ""
}

// formatKeyName converts gocui key constants to readable names dynamically
func (h *ManPageHelpRenderer) formatKeyName(key gocui.Key, mod gocui.Modifier) string {
	var parts []string

	// Add modifiers - note: gocui may not have all these modifiers
	// For now, we'll just handle ModNone and add basic logic for future modifiers
	if mod != gocui.ModNone {
		// If there are non-zero modifiers in the future, we can handle them here
		// For now, most shortcuts use ModNone
		parts = append(parts, fmt.Sprintf("Mod(%d)", mod))
	}

	// Add key name
	keyName := h.getKeyName(key)
	parts = append(parts, keyName)

	return strings.Join(parts, "+")
}

// getKeyName converts gocui.Key to readable string using dynamic lookup
func (h *ManPageHelpRenderer) getKeyName(key gocui.Key) string {
	// Define a mapping for special keys
	keyNames := map[gocui.Key]string{
		gocui.KeyF1:          "F1",
		gocui.KeyF2:          "F2",
		gocui.KeyF3:          "F3",
		gocui.KeyF4:          "F4",
		gocui.KeyF5:          "F5",
		gocui.KeyF6:          "F6",
		gocui.KeyF7:          "F7",
		gocui.KeyF8:          "F8",
		gocui.KeyF9:          "F9",
		gocui.KeyF10:         "F10",
		gocui.KeyF11:         "F11",
		gocui.KeyF12:         "F12",
		gocui.KeyCtrlC:       "Ctrl+C",
		gocui.KeyPgup:        "PgUp",
		gocui.KeyPgdn:        "PgDn",
		gocui.KeyArrowUp:     "↑",
		gocui.KeyArrowDown:   "↓",
		gocui.KeyArrowLeft:   "←",
		gocui.KeyArrowRight:  "→",
		gocui.KeyHome:        "Home",
		gocui.KeyEnd:         "End",
		gocui.KeyEnter:       "Enter",
		gocui.KeySpace:       "Space",
		gocui.KeyTab:         "Tab",
		gocui.KeyEsc:         "Esc",
		gocui.KeyBackspace:   "Backspace",
		gocui.KeyDelete:      "Del",
		gocui.KeyInsert:      "Ins",
		gocui.MouseWheelUp:   "Mouse Wheel Up",
		gocui.MouseWheelDown: "Mouse Wheel Down",
		gocui.MouseLeft:      "Left Click",
		gocui.MouseMiddle:    "Middle Click",
		gocui.MouseRight:     "Right Click",
	}

	// Check if we have a name for this key
	if name, exists := keyNames[key]; exists {
		return name
	}

	// For character keys, try to convert to readable character
	if key >= 32 && key <= 126 {
		return strings.ToUpper(string(rune(key)))
	}

	// Fallback for unknown keys
	return fmt.Sprintf("Key(%d)", key)
}

// generateExamples creates examples by analyzing available commands and shortcuts
func (h *ManPageHelpRenderer) generateExamples() string {
	var sb strings.Builder

	// Get keymap entries for examples
	entries := h.keymap.GetEntries()

	sb.WriteString("### Commands with Shortcuts\n")
	// Show examples for commands that have shortcuts
	for _, entry := range entries {
		if entry.Action.Type == "command" {
			cmd := h.registry.GetCommand(entry.Action.CommandName)
			if cmd != nil {
				keyName := h.formatKeyName(entry.Key, entry.Mod)
				sb.WriteString(fmt.Sprintf("- `:%s` or `%s` - %s\n",
					cmd.GetName(), keyName, cmd.GetDescription()))
			}
		}
	}

	sb.WriteString("\n### Navigation Shortcuts\n")
	// Show examples for direct action shortcuts
	for _, entry := range entries {
		if entry.Action.Type == "function" {
			keyName := h.formatKeyName(entry.Key, entry.Mod)
			sb.WriteString(fmt.Sprintf("- `%s` - %s\n",
				keyName, entry.Description))
		}
	}

	sb.WriteString("\n")

	return sb.String()
}

// RenderSlashCommands generates slash commands help
func (h *ManPageHelpRenderer) RenderSlashCommands() string {
	var sb strings.Builder

	// Header
	sb.WriteString("\n# SLASH COMMANDS\n\n")

	// Get all slash commands
	commandNames := h.slashCommandManager.GetCommandNames()
	if len(commandNames) == 0 {
		sb.WriteString("**No slash commands found.**\n\n")
		sb.WriteString("Slash commands can be defined as markdown files in:\n")
		sb.WriteString("- `.genie/commands/` (project-specific)\n")
		sb.WriteString("- `.claude/commands/` (project-specific)\n")
		sb.WriteString("- `~/.genie/commands/` (user-global)\n")
		sb.WriteString("- `~/.claude/commands/` (user-global)\n\n")
		return sb.String()
	}

	// Sort commands alphabetically
	sort.Strings(commandNames)

	// Quick reference list FIRST
	sb.WriteString("**Available Commands:**\n")
	for _, name := range commandNames {
		sb.WriteString(fmt.Sprintf("- `/%s`\n", name))
	}
	sb.WriteString("\n---\n\n")

	// Separate project and user commands for detailed view
	projectCommands := make([]string, 0)
	userCommands := make([]string, 0)

	for _, name := range commandNames {
		if cmd, exists := h.slashCommandManager.GetCommand(name); exists {
			if cmd.Source == "project" {
				projectCommands = append(projectCommands, name)
			} else {
				userCommands = append(userCommands, name)
			}
		}
	}

	// Show project commands
	if len(projectCommands) > 0 {
		sb.WriteString("## Project Commands\n\n")
		for _, name := range projectCommands {
			if cmd, exists := h.slashCommandManager.GetCommand(name); exists {
				sb.WriteString(h.formatSlashCommand(name, cmd))
			}
		}
	}

	// Show user commands
	if len(userCommands) > 0 {
		sb.WriteString("## User Commands\n\n")
		for _, name := range userCommands {
			if cmd, exists := h.slashCommandManager.GetCommand(name); exists {
				sb.WriteString(h.formatSlashCommand(name, cmd))
			}
		}
	}

	// Usage information
	sb.WriteString("## USAGE\n")
	sb.WriteString("To use a slash command, type `/` followed by the command name:\n\n")
	sb.WriteString("```\n")
	if len(commandNames) > 0 {
		sb.WriteString(fmt.Sprintf("/%s\n", commandNames[0]))
	}
	sb.WriteString("```\n\n")

	sb.WriteString("Commands can accept arguments that will be expanded into the command template using `$ARGUMENTS`.\n\n")

	sb.WriteString("## COMMAND LOCATIONS\n")
	sb.WriteString("Slash commands are loaded from these directories (in order):\n")
	sb.WriteString("1. `.genie/commands/` - Project-specific commands\n")
	sb.WriteString("2. `.claude/commands/` - Project-specific commands (Claude compatibility)\n")
	sb.WriteString("3. `~/.genie/commands/` - User-global commands\n")
	sb.WriteString("4. `~/.claude/commands/` - User-global commands (Claude compatibility)\n\n")

	return sb.String()
}

// formatSlashCommand formats a single slash command for help display
func (h *ManPageHelpRenderer) formatSlashCommand(name string, cmd slashcommands.SlashCommand) string {
	var sb strings.Builder

	// Command header
	sb.WriteString(fmt.Sprintf("### /%s\n", name))

	// Extract first line as description if available
	description := cmd.Description
	lines := strings.Split(description, "\n")
	if len(lines) > 0 && lines[0] != "" {
		firstLine := strings.TrimSpace(lines[0])
		// Truncate long first lines
		if len(firstLine) > 80 {
			firstLine = firstLine[:77] + "..."
		}
		sb.WriteString(fmt.Sprintf("**%s**\n\n", firstLine))
	}

	// Show truncated command content in code block
	contentPreview := description
	if len(contentPreview) > 200 {
		// Find a good truncation point (end of word)
		truncateAt := 197
		for i := 197; i > 150; i-- {
			if contentPreview[i] == ' ' || contentPreview[i] == '\n' {
				truncateAt = i
				break
			}
		}
		contentPreview = contentPreview[:truncateAt] + "..."
	}

	// Show command content in code block
	sb.WriteString("```\n")
	sb.WriteString(contentPreview)
	sb.WriteString("\n```\n\n")

	// Source info
	sb.WriteString(fmt.Sprintf("*Source: %s*\n\n", cmd.Source))

	return sb.String()
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

