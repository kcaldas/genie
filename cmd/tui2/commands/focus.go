package commands

import (
	"fmt"
)

type FocusCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewFocusCommand(ctx *CommandContext) *FocusCommand {
	return &FocusCommand{
		BaseCommand: BaseCommand{
			Name:        "focus",
			Description: "Focus on a specific panel (messages, input, debug)",
			Usage:       ":focus <panel>",
			Examples: []string{
				":focus input",
				":focus messages",
				":focus debug",
			},
			Aliases:  []string{},
			Category: "Navigation",
		},
		ctx: ctx,
	}
}

func (c *FocusCommand) Execute(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: :focus <panel>")
	}

	panelName := args[0]
	return c.ctx.SetCurrentView(panelName)
}