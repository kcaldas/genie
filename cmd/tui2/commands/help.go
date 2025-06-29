package commands

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
	// Determine category to show (if any)
	category := ""
	if len(args) > 0 {
		category = args[0]
	}
	
	// Show help dialog instead of adding to chat
	return c.ctx.ShowHelpDialog(category)
}