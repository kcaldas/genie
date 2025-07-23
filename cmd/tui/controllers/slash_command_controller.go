package controllers

import (
	"fmt"
	"strings"

	os "os"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/slashcommands"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/mitchellh/go-homedir"
)

type SlashCommandController struct {
	commandEventBus     *events.CommandEventBus
	slashCommandManager *slashcommands.Manager
	notification        types.Notification
}

func NewSlashCommandController(commandEventBus *events.CommandEventBus, slashCommandManager *slashcommands.Manager, notification types.Notification) *SlashCommandController {
	controller := &SlashCommandController{
		commandEventBus:     commandEventBus,
		slashCommandManager: slashCommandManager,
		notification:        notification,
	}

	// Discover commands on startup
	projectRoot, err := os.Getwd()
	if err != nil {
		controller.notification.AddSystemMessage(fmt.Sprintf("Error getting current working directory: %v", err))
		return controller
	}

	err = slashCommandManager.DiscoverCommands(projectRoot, homedir.Dir)
	if err != nil {
		controller.notification.AddSystemMessage(fmt.Sprintf("Error discovering slash commands: %v", err))
	}

	// Subscribe to user slash command events
	commandEventBus.Subscribe("user.input.slashcommand", func(event interface{}) {
		controller.notification.AddSystemMessage(fmt.Sprintf("Command executing %v", event))
		if command, ok := event.(string); ok {
			controller.HandleSlashCommand(command)
		}
	})

	return controller
}

func (s *SlashCommandController) HandleSlashCommand(rawCommand string) {
	// Remove the leading '/' and split into command name and args
	input := strings.TrimPrefix(rawCommand, "/")
	parts := strings.Fields(input)

	if len(parts) == 0 {
		return
	}

	cmdName := parts[0]
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}

	slashCmd, ok := s.slashCommandManager.GetCommand(cmdName)
	if !ok {
		s.notification.AddSystemMessage(fmt.Sprintf("Unknown slash command: /%s", cmdName))
		return
	}

	expandedCommand, err := slashCmd.Expand(args)
	if err != nil {
		s.notification.AddSystemMessage(fmt.Sprintf("Error expanding slash command /%s: %v", cmdName, err))
		return
	}

	// Emit the expanded command as a regular text message
	s.commandEventBus.Emit("user.input.text", expandedCommand)
}

