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
			Description: "Expand the prompt to the full height for composing longer messages",
			Usage:       ":write\n\nF4 toggles the same expansion; Ctrl+Shift+V pastes into the expanded prompt. With VimMode on the expanded prompt uses vi keys (:w sends, :q collapses).",
			Examples: []string{
				":write",
				":w",
				"F4 (shortcut)",
				"Ctrl+Shift+V (paste and expand)",
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
