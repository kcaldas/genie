package commands

// ContextViewer opens (or closes) the LLM context viewer.
type ContextViewer interface {
	Show() error
}

type ContextCommand struct {
	BaseCommand
	controller ContextViewer
}

func NewContextCommand(controller ContextViewer) *ContextCommand {
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
