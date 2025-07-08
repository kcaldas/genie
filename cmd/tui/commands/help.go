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
	// Toggle help - if text viewer is visible and showing help, hide it; otherwise show help
	// We need access to app state to check if right panel is visible with text-viewer mode
	if err := c.ctx.ToggleHelpInTextViewer(); err != nil {
		return err
	}
	
	// Refresh the UI to show the changes
	return c.ctx.RefreshUI()
}