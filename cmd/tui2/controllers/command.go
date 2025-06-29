package controllers

import (
	"fmt"
	"strings"
)

type SlashCommandHandler struct {
	registry *CommandRegistry
	// Keep legacy fields for backward compatibility during transition
	commands map[string]CommandFunc
	aliases  map[string]string
}

type CommandFunc func(args []string) error

func NewSlashCommandHandler() *SlashCommandHandler {
	return &SlashCommandHandler{
		registry: NewCommandRegistry(),
		commands: make(map[string]CommandFunc),
		aliases:  make(map[string]string),
	}
}

// RegisterCommandWithMetadata registers a command with full metadata
func (h *SlashCommandHandler) RegisterCommandWithMetadata(cmd *Command) {
	h.registry.Register(cmd)
}

// RegisterCommand registers a command (legacy method for backward compatibility)
func (h *SlashCommandHandler) RegisterCommand(name string, fn CommandFunc) {
	h.commands[name] = fn
}

// RegisterAlias registers an alias (legacy method for backward compatibility)
func (h *SlashCommandHandler) RegisterAlias(alias, command string) {
	h.aliases[alias] = command
}

func (h *SlashCommandHandler) HandleCommand(command string, args []string) error {
	cmd := strings.TrimPrefix(command, "/")
	
	// First try the new registry
	if registeredCmd := h.registry.GetCommand(cmd); registeredCmd != nil {
		return registeredCmd.Handler(args)
	}
	
	// Fall back to legacy system
	if actualCmd, ok := h.aliases[cmd]; ok {
		cmd = actualCmd
	}
	
	if fn, ok := h.commands[cmd]; ok {
		return fn(args)
	}
	
	return fmt.Errorf("unknown command: /%s", cmd)
}

func (h *SlashCommandHandler) GetAvailableCommands() []string {
	// Combine registry and legacy commands
	var commands []string
	
	// Add commands from registry
	for _, cmd := range h.registry.GetAllCommands() {
		commands = append(commands, cmd.Name)
	}
	
	// Add legacy commands
	for cmd := range h.commands {
		commands = append(commands, cmd)
	}
	
	return commands
}

// GetCommandHelp returns help information (now uses registry when available)
func (h *SlashCommandHandler) GetCommandHelp() map[string]string {
	help := make(map[string]string)
	
	// Add commands from registry
	for _, cmd := range h.registry.GetAllCommands() {
		help[cmd.Name] = cmd.Description
		
		// Add aliases with reference to main command
		for _, alias := range cmd.Aliases {
			help[alias] = fmt.Sprintf("Alias for /%s - %s", cmd.Name, cmd.Description)
		}
	}
	
	// Fall back to legacy help for any remaining commands
	legacyHelp := map[string]string{
		"help":   "Show this help message",
		"clear":  "Clear the conversation history",
		"debug":  "Toggle debug panel visibility",
		"config": "Configure TUI settings (cursor, markdown, theme, wrap, timestamps, output)",
		"exit":   "Exit the application",
		"quit":   "Exit the application (alias for /exit)",
		"theme":  "Change the color theme",
		"focus":  "Focus on a specific panel (messages, input, debug)",
		"toggle": "Toggle between layout modes (normal/half/full)",
		"layout": "Set layout mode (normal, half, full)",
		"export": "Export conversation to file",
		"save":   "Save current conversation",
	}
	
	// Add legacy help for commands not in registry
	for cmd, desc := range legacyHelp {
		if _, exists := help[cmd]; !exists {
			help[cmd] = desc
		}
	}
	
	return help
}

// GetRegistry returns the command registry for advanced help features
func (h *SlashCommandHandler) GetRegistry() *CommandRegistry {
	return h.registry
}

// GetCommand returns a command by name or alias
func (h *SlashCommandHandler) GetCommand(name string) *Command {
	return h.registry.GetCommand(name)
}