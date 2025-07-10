package commands

type LayoutCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewLayoutCommand(ctx *CommandContext) *LayoutCommand {
	return &LayoutCommand{
		BaseCommand: BaseCommand{
			Name:        "layout",
			Description: "Show layout information",
			Usage:       ":layout",
			Examples: []string{
				":layout",
			},
			Aliases:  []string{},
			Category: "Layout",
		},
		ctx: ctx,
	}
}

func (c *LayoutCommand) Execute(args []string) error {
	// Layout command simplified since screenManager is removed
	c.ctx.ChatController.AddSystemMessage("Layout uses simple 5-panel system. Use :focus to switch between panels.")
	return nil
}
