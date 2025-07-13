package commands

import "github.com/kcaldas/genie/cmd/events"

type ExitCommand struct {
	BaseCommand
	commandEventBus *events.CommandEventBus
}

func NewExitCommand(commandEventBus *events.CommandEventBus) *ExitCommand {
	return &ExitCommand{
		BaseCommand: BaseCommand{
			Name:        "exit",
			Description: "Exit the application",
			Usage:       ":exit",
			Examples: []string{
				":exit",
			},
			Aliases:  []string{"quit", "q"},
			Category: "General",
		},
		commandEventBus: commandEventBus,
	}
}

func (c *ExitCommand) Execute(args []string) error {
	c.commandEventBus.Emit("app.exit", "")
	return nil
}

