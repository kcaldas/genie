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
			Aliases:   []string{"h", "?"},
			Category:  "General",
			Shortcuts: []string{"shortcut.help"},
		},
		ctx: ctx,
	}
}

func (c *HelpCommand) Execute(args []string) error {
	if err := c.ctx.ToggleHelpInTextViewer(); err != nil {
		return err
	}

	return nil
}

