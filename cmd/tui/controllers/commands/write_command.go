package commands

// WriteOpener opens the write-mode composer.
type WriteOpener interface {
	Show() error
}

type WriteCommand struct {
	BaseCommand
	controller WriteOpener
}

func NewWriteCommand(controller WriteOpener) *WriteCommand {
	return &WriteCommand{
		BaseCommand: BaseCommand{
			Name:        "write",
			Description: "Open write component for composing longer messages",
			Usage:       ":write\n\nPaste shortcuts:\n• Ctrl+V, Alt+V, Shift+Insert: Auto-detect multiline paste\n• Ctrl+Shift+V: Force write component with clipboard",
			Examples: []string{
				":write",
				":w",
				"F4 (shortcut)",
				"Ctrl+Shift+V (paste to write)",
			},
			Aliases:  []string{"w"},
			Category: "General",
		},
		controller: controller,
	}
}

func (c *WriteCommand) Execute(args []string) error {
	return c.controller.Show()
}
