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
		config := c.ctx.GuiCommon.GetConfig()
		currentTheme := config.Theme
		outputMode := config.OutputMode
		glamourStyle := presentation.GetGlamourStyleForTheme(currentTheme)
		
		content := fmt.Sprintf("Available themes: %s\n\nCurrent theme: %s\nOutput mode: %s\nMarkdown style: %s\n\nUsage: :theme <name>",
			strings.Join(themes, ", "),
			currentTheme,
			outputMode,
			glamourStyle)

		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: content,
		})
		return c.ctx.RefreshUI()
	}

	themeName := args[0]
	
	// Validate theme exists  
	if theme := presentation.GetTheme(themeName); theme == nil {
		availableThemes := strings.Join(presentation.GetThemeNames(), ", ")
		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Unknown theme: %s. Available themes: %s", themeName, availableThemes),
		})
		return c.ctx.RefreshUI()
	}
	
	// Update config
	config := c.ctx.GuiCommon.GetConfig()
	config.Theme = themeName
	
	// Save config
	if err := c.ctx.ConfigHelper.Save(config); err != nil {
		c.ctx.StateAccessor.AddDebugMessage(fmt.Sprintf("Failed to save config: %v", err))
	}
	
	// Apply theme changes to the running application
	if err := c.ctx.RefreshTheme(); err != nil {
		c.ctx.StateAccessor.AddDebugMessage(fmt.Sprintf("Failed to refresh theme: %v", err))
		return c.ctx.RefreshUI()
	}
	
	// Success message
	c.ctx.StateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: fmt.Sprintf("Theme changed to %s", themeName),
	})
	
	// Final UI refresh to show the theme change message
	return c.ctx.RefreshUI()
}