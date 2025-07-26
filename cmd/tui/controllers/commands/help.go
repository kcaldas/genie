package commands

import (
	"github.com/kcaldas/genie/cmd/tui/controllers"
)

type HelpCommand struct {
	BaseCommand
	helpController controllers.HelpControllerInterface
}

func NewHelpCommand(helpController controllers.HelpControllerInterface) *HelpCommand {
	return &HelpCommand{
		BaseCommand: BaseCommand{
			Name:        "help",
			Description: "Show help message with available commands and shortcuts",
			Usage:       ":help [command]",
			Examples: []string{
				":help",
				":?",
				":help /",
				":help slash",
				":help config",
				":help theme",
			},
			Aliases:   []string{"h", "?"},
			Category:  "General",
			Shortcuts: []string{"shortcut.help"},
		},
		helpController: helpController,
	}
}

func (c *HelpCommand) Execute(args []string) error {
	// Check if user wants slash command help
	if len(args) > 0 && (args[0] == "/" || args[0] == "slash") {
		if err := c.helpController.ShowSlashCommandsHelp(); err != nil {
			return err
		}
		return nil
	}

	// Default help behavior
	if err := c.helpController.ToggleHelp(); err != nil {
		return err
	}

	return nil
}

