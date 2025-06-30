package commands

import (
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type HelpCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewHelpCommand(ctx *CommandContext) *HelpCommand {
	return &HelpCommand{
		BaseCommand: BaseCommand{
			Name:        "help",
			Description: "Show help message with available commands and shortcuts",
			Usage:       ":help [command]",
			Examples: []string{
				":help",
				":?",
				":help config",
				":help theme",
			},
			Aliases:  []string{"h", "?"},
			Category: "General",
		},
		ctx: ctx,
	}
}

func (c *HelpCommand) Execute(args []string) error {
	// Get rendered help text and display as system message
	helpText := c.ctx.GetHelpText()
	
	// Add help text as system message with markdown content type
	c.ctx.StateAccessor.AddMessage(types.Message{
		Role:        "system",
		Content:     helpText,
		ContentType: "markdown",
	})
	
	// Refresh UI to show the new message
	return c.ctx.RefreshUI()
}