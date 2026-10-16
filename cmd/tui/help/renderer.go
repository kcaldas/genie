// Package help renders the man-page style help shown by F1 and :help from
// the command registry, the active shortcuts and the slash commands.
package help

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kcaldas/genie/cmd/slashcommands"
	"github.com/kcaldas/genie/cmd/tui/controllers/commands"
)

// Shortcut is one keyboard shortcut as the help page shows it. Command
// names the ':' command the key runs, or is empty for a direct action.
type Shortcut struct {
	Key         string
	Description string
	Command     string
}

// Renderer generates man-style help documentation.
type Renderer interface {
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

// ManPageHelpRenderer implements Renderer with man-page style formatting.
type ManPageHelpRenderer struct {
	registry            *commands.CommandRegistry
	shortcuts           []Shortcut
	slashCommandManager *slashcommands.Manager
}

// NewManPageHelpRenderer creates a help renderer over the given registry,
// shortcuts and slash commands.
func NewManPageHelpRenderer(registry *commands.CommandRegistry, shortcuts []Shortcut, slashCommandManager *slashcommands.Manager) Renderer {
	return &ManPageHelpRenderer{
		registry:            registry,
		shortcuts:           shortcuts,
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

		fmt.Fprintf(&sb, "### %s\n", category)

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

// RenderShortcuts generates the SHORTCUTS section, sorted by key name.
func (h *ManPageHelpRenderer) RenderShortcuts() string {
	var sb strings.Builder

	sorted := append([]Shortcut(nil), h.shortcuts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	for _, s := range sorted {
		description := s.Description
		if s.Command != "" {
			if cmd := h.registry.GetCommand(s.Command); cmd != nil {
				description = cmd.GetDescription()
			}
			description = fmt.Sprintf("%s (same as `:%s`)", description, s.Command)
		}
		fmt.Fprintf(&sb, "- `%s` - %s\n", s.Key, description)
	}

	return sb.String()
}

// formatCommand formats a single command in man-page style
func (h *ManPageHelpRenderer) formatCommand(cmd *commands.CommandWrapper) string {
	var sb strings.Builder

	// Command name with aliases
	fmt.Fprintf(&sb, "**:%s**", cmd.GetName())
	if len(cmd.GetAliases()) > 0 {
		aliasStr := strings.Join(cmd.GetAliases(), ", :")
		fmt.Fprintf(&sb, " (:%s)", aliasStr)
	}

	// Add shortcut if this command has one
	if shortcut := h.findShortcutForCommand(cmd.GetName()); shortcut != "" {
		fmt.Fprintf(&sb, " [%s]", shortcut)
	}

	sb.WriteString("  \n")

	// Description
	if cmd.GetDescription() != "" {
		fmt.Fprintf(&sb, "    %s  \n", cmd.GetDescription())
	}

	// Usage
	if cmd.GetUsage() != "" {
		fmt.Fprintf(&sb, "\nUsage: `%s`  \n", cmd.GetUsage())
	}

	// Examples
	if len(cmd.GetExamples()) > 0 {
		sb.WriteString("\nExamples:  \n  \n")
		for _, example := range cmd.GetExamples() {
			fmt.Fprintf(&sb, "- `%s`  \n", example)
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// findShortcutForCommand returns the key bound to a command, or "".
func (h *ManPageHelpRenderer) findShortcutForCommand(commandName string) string {
	for _, s := range h.shortcuts {
		if s.Command == commandName {
			return s.Key
		}
	}
	return ""
}

// generateExamples creates examples by analyzing available commands and shortcuts
func (h *ManPageHelpRenderer) generateExamples() string {
	var sb strings.Builder

	sb.WriteString("### Commands with Shortcuts\n")
	for _, s := range h.shortcuts {
		if s.Command == "" {
			continue
		}
		if cmd := h.registry.GetCommand(s.Command); cmd != nil {
			fmt.Fprintf(&sb, "- `:%s` or `%s` - %s\n", cmd.GetName(), s.Key, cmd.GetDescription())
		}
	}

	sb.WriteString("\n### Navigation Shortcuts\n")
	for _, s := range h.shortcuts {
		if s.Command == "" {
			fmt.Fprintf(&sb, "- `%s` - %s\n", s.Key, s.Description)
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
		fmt.Fprintf(&sb, "- `/%s`\n", name)
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
		fmt.Fprintf(&sb, "/%s\n", commandNames[0])
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
	fmt.Fprintf(&sb, "### /%s\n", name)

	// Extract first line as description if available
	description := cmd.Description
	lines := strings.Split(description, "\n")
	if len(lines) > 0 && lines[0] != "" {
		firstLine := strings.TrimSpace(lines[0])
		// Truncate long first lines
		if len(firstLine) > 80 {
			firstLine = firstLine[:77] + "..."
		}
		fmt.Fprintf(&sb, "**%s**\n\n", firstLine)
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
	fmt.Fprintf(&sb, "*Source: %s*\n\n", cmd.Source)

	return sb.String()
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
