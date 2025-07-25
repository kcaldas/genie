package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type CommandHandler struct {
	registry *CommandRegistry
	// Keep legacy fields for backward compatibility during transition
	commands map[string]CommandFunc
	aliases  map[string]string
	// Event bus for direct command subscription
	commandEventBus *events.CommandEventBus
	notification    types.Notification
}

type CommandFunc func(args []string) error

func NewCommandHandler(commandEventBus *events.CommandEventBus, notification types.Notification, registry *CommandRegistry) *CommandHandler {
	handler := &CommandHandler{
		registry:        registry,
		commands:        make(map[string]CommandFunc),
		aliases:         make(map[string]string),
		commandEventBus: commandEventBus,
		notification:    notification,
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
func (h *CommandHandler) RegisterNewCommand(cmd Command) {
	h.registry.RegisterNewCommand(cmd)

	// Auto-subscribe to command's declared shortcut events
	for _, shortcut := range cmd.GetShortcuts() {
		// Capture cmd and shortcut for closure
		capturedCmd := cmd
		h.commandEventBus.Subscribe(shortcut, func(event interface{}) {
			capturedCmd.Execute([]string{})
		})
	}
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
		err := registeredCmd.Execute(args)
		if err == nil {
			// Publish command completion event
			h.commandEventBus.Emit("command."+cmd+".executed", map[string]interface{}{
				"command": cmd,
				"args":    args,
			})
		}
		return err
	}

	// Try vim-style prefix matching for compound commands like "y1k" (only if not exact match)
	if vimCmd, vimArgs := h.tryVimStyleParsing(cmd, args); vimCmd != nil {
		err := vimCmd.Execute(vimArgs)
		if err == nil {
			// Publish command completion event using the base command name
			h.commandEventBus.Emit("command."+vimCmd.GetName()+".executed", map[string]interface{}{
				"command": vimCmd.GetName(),
				"args":    vimArgs,
			})
		}
		return err
	}

	// Fall back to legacy system
	if actualCmd, ok := h.aliases[cmd]; ok {
		cmd = actualCmd
	}

	if fn, ok := h.commands[cmd]; ok {
		err := fn(args)
		if err == nil {
			// Publish command completion event for legacy commands
			h.commandEventBus.Emit("command."+cmd+".executed", map[string]interface{}{
				"command": cmd,
				"args":    args,
			})
		}
		return err
	}

	// Handle unknown command gracefully instead of returning error
	h.notification.AddSystemMessage(fmt.Sprintf("Unknown command: %s. Type :? for available commands.", cmd))
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
