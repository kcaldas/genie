package controllers

import (
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/commands"
)

type CommandHandler struct {
	registry *CommandRegistry
	// Keep legacy fields for backward compatibility during transition
	commands map[string]CommandFunc
	aliases  map[string]string
	// Callback for handling unknown commands
	unknownCommandHandler func(commandName string)
	// Event bus for direct command subscription
	commandEventBus *events.CommandEventBus
}

type CommandFunc func(args []string) error

func NewCommandHandler(commandEventBus *events.CommandEventBus) *CommandHandler {
	handler := &CommandHandler{
		registry:        NewCommandRegistry(),
		commands:        make(map[string]CommandFunc),
		aliases:         make(map[string]string),
		commandEventBus: commandEventBus,
	}

	// Subscribe to user command events
	commandEventBus.Subscribe("user.input.command", func(event interface{}) {
		if command, ok := event.(string); ok {
			handler.handleCommandEvent(command)
		}
	})

	return handler
}

// RegisterNewCommand registers a new command interface
func (h *CommandHandler) RegisterNewCommand(cmd commands.Command) {
	h.registry.RegisterNewCommand(cmd)
}

// GetRegistry returns the command registry
func (h *CommandHandler) GetRegistry() *CommandRegistry {
	return h.registry
}

// RegisterCommand registers a command (legacy method for backward compatibility)
func (h *CommandHandler) RegisterCommand(name string, fn CommandFunc) {
	h.commands[name] = fn
}

// RegisterAlias registers an alias (legacy method for backward compatibility)
func (h *CommandHandler) RegisterAlias(alias, command string) {
	h.aliases[alias] = command
}

// handleCommandEvent handles commands received from the event bus
func (h *CommandHandler) handleCommandEvent(command string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	args := parts[1:]

	// Handle the command (ignore errors for now - they're handled in HandleCommand)
	h.HandleCommand(cmd, args)
}

func (h *CommandHandler) HandleCommand(command string, args []string) error {
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
func (h *CommandHandler) tryVimStyleParsing(cmd string, args []string) (*CommandWrapper, []string) {
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
func (h *CommandHandler) SetUnknownCommandHandler(handler func(string)) {
	h.unknownCommandHandler = handler
}

// handleUnknownCommand handles unknown commands by calling the registered handler
func (h *CommandHandler) handleUnknownCommand(commandName string) {
	if h.unknownCommandHandler != nil {
		h.unknownCommandHandler(commandName)
	}
}

func (h *CommandHandler) GetAvailableCommands() []string {
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
func (h *CommandHandler) GetCommand(name string) *CommandWrapper {
	return h.registry.GetCommand(name)
}
