package controllers

import (
	"strings"

	"github.com/kcaldas/genie/cmd/tui2/commands"
)

type SlashCommandHandler struct {
	registry *CommandRegistry
	// Keep legacy fields for backward compatibility during transition
	commands map[string]CommandFunc
	aliases  map[string]string
	// Callback for handling unknown commands
	unknownCommandHandler func(commandName string)
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

// Register registers a legacy command
func (h *SlashCommandHandler) Register(cmd *Command) {
	h.registry.Register(cmd)
}

// RegisterNewCommand registers a new command interface
func (h *SlashCommandHandler) RegisterNewCommand(cmd commands.Command) {
	h.registry.RegisterNewCommand(cmd)
}

// GetRegistry returns the command registry
func (h *SlashCommandHandler) GetRegistry() *CommandRegistry {
	return h.registry
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
	cmd := strings.TrimPrefix(command, ":")

	// First try exact match in the new registry
	if registeredCmd := h.registry.GetCommand(cmd); registeredCmd != nil {
		return registeredCmd.Execute(args)
	}

	// Try vim-style prefix matching for compound commands like "y1k" (only if not exact match)
	if vimCmd, vimArgs := h.tryVimStyleParsing(cmd, args); vimCmd != nil {
		return vimCmd.Execute(vimArgs)
	}

	// Fall back to legacy system
	if actualCmd, ok := h.aliases[cmd]; ok {
		cmd = actualCmd
	}

	if fn, ok := h.commands[cmd]; ok {
		return fn(args)
	}

	// Handle unknown command gracefully instead of returning error
	h.handleUnknownCommand(cmd)
	return nil
}

// tryVimStyleParsing attempts to parse vim-style compound commands like "y1k" into "y" + "1k"
func (h *SlashCommandHandler) tryVimStyleParsing(cmd string, args []string) (*CommandWrapper, []string) {
	// Define vim-style commands that support compound syntax
	// Order by length (longest first) to avoid "y" matching "yank" commands
	vimCommands := []string{"yank", "y"}

	for _, vimCmd := range vimCommands {
		// Check if cmd starts with the vim command
		if strings.HasPrefix(cmd, vimCmd) && len(cmd) > len(vimCmd) {
			// Extract the suffix (e.g., "1k" from "y1k")
			suffix := cmd[len(vimCmd):]

			// Check if the vim command exists in registry
			if registeredCmd := h.registry.GetCommand(vimCmd); registeredCmd != nil {
				// Combine suffix with existing args
				newArgs := append([]string{suffix}, args...)
				return registeredCmd, newArgs
			}
		}
	}

	return nil, nil
}

// SetUnknownCommandHandler sets the callback for handling unknown commands
func (h *SlashCommandHandler) SetUnknownCommandHandler(handler func(string)) {
	h.unknownCommandHandler = handler
}

// handleUnknownCommand handles unknown commands by calling the registered handler
func (h *SlashCommandHandler) handleUnknownCommand(commandName string) {
	if h.unknownCommandHandler != nil {
		h.unknownCommandHandler(commandName)
	}
}

func (h *SlashCommandHandler) GetAvailableCommands() []string {
	// Combine registry and legacy commands
	var commands []string

	// Add commands from registry
	for _, cmd := range h.registry.GetAllCommands() {
		commands = append(commands, cmd.GetName())
	}

	// Add legacy commands
	for cmd := range h.commands {
		commands = append(commands, cmd)
	}

	return commands
}

// GetCommand returns a command by name or alias
func (h *SlashCommandHandler) GetCommand(name string) *CommandWrapper {
	return h.registry.GetCommand(name)
}

