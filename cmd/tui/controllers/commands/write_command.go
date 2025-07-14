package commands

import "github.com/kcaldas/genie/cmd/tui/controllers"

type WriteCommand struct {
	BaseCommand
	controller *controllers.WriteController
}

func NewWriteCommand(controller *controllers.WriteController) *WriteCommand {
	return &WriteCommand{
		BaseCommand: BaseCommand{
			Name:        "write",
			Description: "Open write component for composing longer messages",
			Usage:       ":write",
			Examples: []string{
				":write",
				":w",
			},
			Aliases:  []string{"w"},
			Category: "General",
		},
		controller: controller,
	}
}

func (c *WriteCommand) Execute(args []string) error {
	return c.controller.Show()
}