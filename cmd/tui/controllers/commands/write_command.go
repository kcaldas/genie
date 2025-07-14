package commands

import "github.com/kcaldas/genie/cmd/tui/controllers"

type WriteCommand struct {
	BaseCommand
	controller *controllers.WriteController
}

func NewWriteCommand(controller *controllers.WriteController) *WriteCommand {
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