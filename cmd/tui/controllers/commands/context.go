package commands

type ContextCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewContextCommand(ctx *CommandContext) *ContextCommand {
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
		ctx: ctx,
	}
}

func (c *ContextCommand) Execute(args []string) error {
	// Use LLMContextController directly
	// The toggle behavior is handled by ShowLLMContextViewer for backward compatibility
	// But we can also implement it here if needed
	return c.ctx.ShowLLMContextViewer()
}