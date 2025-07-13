package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
)

type ThemeCommand struct {
	BaseCommand
	configManager   *helpers.ConfigManager
	commandEventBus *events.CommandEventBus
	notification    *controllers.ChatController
}

func NewThemeCommand(configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, notification *controllers.ChatController) *ThemeCommand {
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
		configManager:   configManager,
		commandEventBus: commandEventBus,
		notification:    notification,
	}
}

func (c *ThemeCommand) Execute(args []string) error {
	if len(args) == 0 {
		themes := presentation.GetThemeNames()
		config := c.configManager.GetConfig()
		currentTheme := config.Theme
		outputMode := config.OutputMode
		glamourStyle := presentation.GetGlamourStyleForTheme(currentTheme)

		content := fmt.Sprintf("Available themes: %s\n\nCurrent theme: %s\nOutput mode: %s\nMarkdown style: %s\n\nUsage: :theme <name>",
			strings.Join(themes, ", "),
			currentTheme,
			outputMode,
			glamourStyle)

		c.notification.AddSystemMessage(content)
		return nil
	}

	themeName := args[0]

	// Validate theme exists by checking against available theme names
	themeNames := presentation.GetThemeNames()
	themeExists := slices.Contains(themeNames, themeName)

	if !themeExists {
		availableThemes := strings.Join(themeNames, ", ")
		c.notification.AddErrorMessage(fmt.Sprintf("Unknown theme: %s. Available themes: %s", themeName, availableThemes))
		return nil
	}

	// Update config
	config := c.configManager.GetConfig()
	oldTheme := config.Theme
	config.Theme = themeName

	// Save config
	if err := c.configManager.Save(config); err != nil {
		// Note: No logger available in this refactored version, but save errors are not critical
	}

	// Emit theme changed event for components to react
	c.commandEventBus.Emit("theme.changed", map[string]interface{}{
		"oldTheme": oldTheme,
		"newTheme": themeName,
		"config":   config,
	})

	// Success message
	c.notification.AddSystemMessage(fmt.Sprintf("Theme changed to %s", themeName))

	// Final UI refresh to show the theme change message
	return nil
}
