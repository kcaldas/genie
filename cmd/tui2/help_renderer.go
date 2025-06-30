package tui2

import (
	"fmt"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/controllers"
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
}

// ManPageHelpRenderer implements HelpRenderer with man-page style formatting
type ManPageHelpRenderer struct {
	registry *controllers.CommandRegistry
	keymap   *Keymap
}

// NewManPageHelpRenderer creates a new help renderer
func NewManPageHelpRenderer(registry *controllers.CommandRegistry, keymap *Keymap) HelpRenderer {
	return &ManPageHelpRenderer{
		registry: registry,
		keymap:   keymap,
	}
}

// RenderHelp generates complete man-style help documentation
func (h *ManPageHelpRenderer) RenderHelp() string {
	var sb strings.Builder
	
	// Header
	sb.WriteString("# GENIE(1)\n\n")
	sb.WriteString("## NAME\n")
	sb.WriteString("genie - AI-powered coding assistant with interactive TUI\n\n")
	
	sb.WriteString("## DESCRIPTION\n")
	sb.WriteString("Genie is an interactive terminal user interface for AI-assisted software development.\n")
	sb.WriteString("It supports both vi-style text commands and direct keyboard shortcuts.\n\n")
	
	// Commands section
	sb.WriteString("## COMMANDS\n")
	sb.WriteString("Commands are entered by typing `:` followed by the command name in the input field.\n\n")
	sb.WriteString(h.RenderCommandsByCategory())
	
	// Shortcuts section
	sb.WriteString("\n## SHORTCUTS\n")
	sb.WriteString("Keyboard shortcuts work globally and provide quick access to common actions.\n\n")
	sb.WriteString(h.RenderShortcuts())
	
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
	
	// Dynamically group shortcuts by type
	commandShortcuts := make([]KeymapEntry, 0)
	directShortcuts := make([]KeymapEntry, 0)
	
	for _, entry := range entries {
		if entry.Action.Type == "command" {
			commandShortcuts = append(commandShortcuts, entry)
		} else {
			directShortcuts = append(directShortcuts, entry)
		}
	}
	
	// Sort shortcuts by key name for consistent output
	sort.Slice(commandShortcuts, func(i, j int) bool {
		return h.formatKeyName(commandShortcuts[i].Key, commandShortcuts[i].Mod) < 
		       h.formatKeyName(commandShortcuts[j].Key, commandShortcuts[j].Mod)
	})
	sort.Slice(directShortcuts, func(i, j int) bool {
		return h.formatKeyName(directShortcuts[i].Key, directShortcuts[i].Mod) < 
		       h.formatKeyName(directShortcuts[j].Key, directShortcuts[j].Mod)
	})
	
	// Command shortcuts section
	if len(commandShortcuts) > 0 {
		sb.WriteString("### Command Shortcuts\n")
		sb.WriteString("These shortcuts execute the same actions as their corresponding commands:\n\n")
		
		for _, entry := range commandShortcuts {
			keyName := h.formatKeyName(entry.Key, entry.Mod)
			commandName := entry.Action.CommandName
			
			// Find the actual command to get more details
			cmd := h.registry.GetCommand(commandName)
			var cmdDescription string
			if cmd != nil {
				cmdDescription = cmd.GetDescription()
			} else {
				cmdDescription = entry.Description
			}
			
			sb.WriteString(fmt.Sprintf("**%s**  \n", keyName))
			sb.WriteString(fmt.Sprintf("    %s (same as `:%s`)  \n", cmdDescription, commandName))
			sb.WriteString("\n")
		}
	}
	
	// Direct action shortcuts section  
	if len(directShortcuts) > 0 {
		sb.WriteString("### Direct Action Shortcuts\n")
		sb.WriteString("These shortcuts provide direct actions without command equivalents:\n\n")
		
		for _, entry := range directShortcuts {
			keyName := h.formatKeyName(entry.Key, entry.Mod)
			sb.WriteString(fmt.Sprintf("**%s**  \n", keyName))
			sb.WriteString(fmt.Sprintf("    %s  \n", entry.Description))
			sb.WriteString("\n")
		}
	}
	
	return sb.String()
}

// formatCommand formats a single command in man-page style
func (h *ManPageHelpRenderer) formatCommand(cmd *controllers.CommandWrapper) string {
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
		sb.WriteString(fmt.Sprintf("    Usage: `%s`  \n", cmd.GetUsage()))
	}
	
	// Examples
	if len(cmd.GetExamples()) > 0 {
		sb.WriteString("    Examples:  \n")
		for _, example := range cmd.GetExamples() {
			sb.WriteString(fmt.Sprintf("        `%s`  \n", example))
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
		gocui.KeyF1:            "F1",
		gocui.KeyF2:            "F2",
		gocui.KeyF3:            "F3",
		gocui.KeyF4:            "F4",
		gocui.KeyF5:            "F5",
		gocui.KeyF6:            "F6",
		gocui.KeyF7:            "F7",
		gocui.KeyF8:            "F8",
		gocui.KeyF9:            "F9",
		gocui.KeyF10:           "F10",
		gocui.KeyF11:           "F11",
		gocui.KeyF12:           "F12",
		gocui.KeyCtrlC:         "C",
		gocui.KeyPgup:          "PgUp",
		gocui.KeyPgdn:          "PgDn",
		gocui.KeyArrowUp:       "↑",
		gocui.KeyArrowDown:     "↓",
		gocui.KeyArrowLeft:     "←",
		gocui.KeyArrowRight:    "→",
		gocui.KeyHome:          "Home",
		gocui.KeyEnd:           "End",
		gocui.KeyEnter:         "Enter",
		gocui.KeySpace:         "Space",
		gocui.KeyTab:           "Tab",
		gocui.KeyEsc:           "Esc",
		gocui.KeyBackspace:     "Backspace",
		gocui.KeyDelete:        "Del",
		gocui.KeyInsert:        "Ins",
		gocui.MouseWheelUp:     "Mouse Wheel Up",
		gocui.MouseWheelDown:   "Mouse Wheel Down",
		gocui.MouseLeft:        "Left Click",
		gocui.MouseMiddle:      "Middle Click",
		gocui.MouseRight:       "Right Click",
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
	
	sb.WriteString("```\n")
	
	// Get keymap entries for examples
	entries := h.keymap.GetEntries()
	
	// Show examples for commands that have shortcuts
	for _, entry := range entries {
		if entry.Action.Type == "command" {
			cmd := h.registry.GetCommand(entry.Action.CommandName)
			if cmd != nil {
				keyName := h.formatKeyName(entry.Key, entry.Mod)
				sb.WriteString(fmt.Sprintf(":%s%s# %s\n", 
					cmd.GetName(),
					strings.Repeat(" ", max(0, 12-len(cmd.GetName()))),
					cmd.GetDescription()))
				sb.WriteString(fmt.Sprintf("%s%s# Same as :%s\n", 
					keyName,
					strings.Repeat(" ", max(0, 12-len(keyName))),
					cmd.GetName()))
			}
		}
	}
	
	// Show examples for direct action shortcuts
	sb.WriteString("\n# Navigation:\n")
	for _, entry := range entries {
		if entry.Action.Type == "function" {
			keyName := h.formatKeyName(entry.Key, entry.Mod)
			sb.WriteString(fmt.Sprintf("%s%s# %s\n", 
				keyName,
				strings.Repeat(" ", max(0, 12-len(keyName))),
				entry.Description))
		}
	}
	
	sb.WriteString("```\n")
	return sb.String()
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}