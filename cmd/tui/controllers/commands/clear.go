package commands

import "github.com/kcaldas/genie/cmd/tui/controllers"

type ClearCommand struct {
	BaseCommand
	controller *controllers.ChatController
}

func NewClearCommand(controller *controllers.ChatController) *ClearCommand {
	return &ClearCommand{
		BaseCommand: BaseCommand{
			Name:        "clear",
			Description: "Clear the conversation history",
			Usage:       ":clear",
			Examples: []string{
				":clear",
			},
			Aliases:  []string{"cls"},
			Category: "Chat",
		},
		controller: controller,
	}
}

func (c *ClearCommand) Execute(args []string) error {
	if err := c.controller.ClearConversation(); err != nil {
		return err
	}
	return nil
}

