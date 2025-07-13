package commands

import "github.com/kcaldas/genie/cmd/tui/controllers"

type ContextCommand struct {
	BaseCommand
	controller *controllers.LLMContextController
}

func NewContextCommand(controller *controllers.LLMContextController) *ContextCommand {
	return &ContextCommand{
		BaseCommand: BaseCommand{
			Name:        "context",
			Description: "Show LLM context viewer with all context parts",
			Usage:       ":context",
			Examples: []string{
				":context",
				":ctx",
			},
			Aliases:  []string{"ctx"},
			Category: "General",
		},
		controller: controller,
	}
}

func (c *ContextCommand) Execute(args []string) error {
	return c.controller.Show()
}

