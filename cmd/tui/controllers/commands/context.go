package commands

import "github.com/kcaldas/genie/cmd/tui/controllers"

type ContextCommand struct {
	BaseCommand
	ctx        *CommandContext
	controller *controllers.LLMContextController
}

func NewContextCommand(ctx *CommandContext, controller *controllers.LLMContextController) *ContextCommand {
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
		ctx:        ctx,
		controller: controller,
	}
}

func (c *ContextCommand) Execute(args []string) error {
	return c.controller.Show()
}

