package commands

type ClearCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewClearCommand(ctx *CommandContext) *ClearCommand {
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
		ctx: ctx,
	}
}

func (c *ClearCommand) Execute(args []string) error {
	if err := c.ctx.ChatController.ClearConversation(); err != nil {
		return err
	}
	return c.ctx.RefreshUI()
}