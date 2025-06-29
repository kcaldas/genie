package commands

import (
	"github.com/kcaldas/genie/cmd/tui2/types"
)

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
	c.ctx.StateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: "Layout uses simple 5-panel system. Use :focus to switch between panels.",
	})
	return c.ctx.RefreshUI()
}

type ToggleCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewToggleCommand(ctx *CommandContext) *ToggleCommand {
	return &ToggleCommand{
		BaseCommand: BaseCommand{
			Name:        "toggle",
			Description: "Toggle command (deprecated)",
			Usage:       ":toggle",
			Examples: []string{
				":toggle",
			},
			Aliases:  []string{},
			Category: "Layout",
		},
		ctx: ctx,
	}
}

func (c *ToggleCommand) Execute(args []string) error {
	// Toggle command removed since screenManager is removed
	c.ctx.StateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: "Toggle command is no longer available",
	})
	return c.ctx.RefreshUI()
}