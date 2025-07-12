package commands

type ExitCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewExitCommand(ctx *CommandContext) *ExitCommand {
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
		ctx: ctx,
	}
}

func (c *ExitCommand) Execute(args []string) error {
	c.ctx.CommandEventBus.Emit("app.exit", "")
	return nil
}

