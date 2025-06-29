package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type ThemeCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewThemeCommand(ctx *CommandContext) *ThemeCommand {
	return &ThemeCommand{
		BaseCommand: BaseCommand{
			Name:        "theme",
			Description: "Change the color theme or list available themes",
			Usage:       ":theme [theme_name]",
			Examples: []string{
				":theme",
				":theme dark",
				":theme light",
				":theme monokai",
			},
			Aliases:  []string{},
			Category: "Configuration",
		},
		ctx: ctx,
	}
}

func (c *ThemeCommand) Execute(args []string) error {
	if len(args) == 0 {
		themes := presentation.GetThemeNames()
		currentTheme := c.ctx.GuiCommon.GetConfig().Theme
		glamourStyle := presentation.GetGlamourStyleForTheme(currentTheme)
		
		content := fmt.Sprintf("Available themes: %s\n\nCurrent theme: %s\nMarkdown style: %s\n\nUsage: :theme <name>",
			strings.Join(themes, ", "),
			currentTheme,
			glamourStyle)

		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: content,
		})
		return c.ctx.RefreshUI()
	}

	themeName := args[0]
	
	// Update config through the existing method
	// This would need to be implemented properly with actual config update
	// For now, we'll use a placeholder
	
	// TODO: Implement proper theme switching
	c.ctx.StateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: fmt.Sprintf("Theme switching to %s - implementation needed", themeName),
	})
	
	return c.ctx.RefreshUI()
}