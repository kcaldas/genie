package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
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
		return nil
	}

	themeName := args[0]
	
	// Validate theme exists by checking against available theme names
	themeNames := presentation.GetThemeNames()
	themeExists := false
	for _, name := range themeNames {
		if name == themeName {
			themeExists = true
			break
		}
	}
	
	if !themeExists {
		availableThemes := strings.Join(themeNames, ", ")
		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Unknown theme: %s. Available themes: %s", themeName, availableThemes),
		})
		return nil
	}
	
	// Update config
	config := c.ctx.GuiCommon.GetConfig()
	config.Theme = themeName
	
	// Save config
	if err := c.ctx.ConfigHelper.Save(config); err != nil {
		c.ctx.StateAccessor.AddDebugMessage(fmt.Sprintf("Config save failed: %v", err))
	}
	
	// Apply theme changes to the running application
	if err := c.ctx.RefreshTheme(); err != nil {
		c.ctx.StateAccessor.AddDebugMessage(fmt.Sprintf("Theme refresh failed: %v", err))
		return nil
	}
	
	// Success message
	c.ctx.StateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: fmt.Sprintf("Theme changed to %s", themeName),
	})
	
	// Final UI refresh to show the theme change message
	return nil
}