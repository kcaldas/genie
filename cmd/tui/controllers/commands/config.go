package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/presentation"
)

type ConfigCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewConfigCommand(ctx *CommandContext) *ConfigCommand {
	return &ConfigCommand{
		BaseCommand: BaseCommand{
			Name:        "config",
			Description: "Configure TUI settings (cursor, markdown, theme, wrap, timestamps, output)",
			Usage:       ":config <setting> <value> | :config reset",
			Examples: []string{
				":config",
				":config theme dark",
				":config markdown-theme dracula",
				":config markdown-theme auto",
				":config cursor true",
				":config markdown false",
				":config output true",
				":config output 256",
				":config output normal",
				":config border false",
				":config messagesborder true",
				":config userlabel >",
				":config assistantlabel ★",
				":config systemlabel ■",
				":config errorlabel ✗",
				":config reset",
			},
			Aliases:  []string{"cfg", "settings"},
			Category: "Configuration",
		},
		ctx: ctx,
	}
}

func (c *ConfigCommand) Execute(args []string) error {
	if len(args) == 0 {
		c.ctx.CommandEventBus.Emit("user.input.command", ":help")
		return nil
	}

	// Handle reset command
	if args[0] == "reset" {
		return c.resetConfig()
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: :config <setting> <value> or :config reset")
	}

	setting := args[0]
	value := strings.Join(args[1:], " ")

	return c.updateConfig(setting, value)
}

func (c *ConfigCommand) updateConfig(setting, value string) error {
	// Validate output mode before updating config
	if setting == "output" || setting == "outputmode" {
		if !(value == "true" || value == "256" || value == "normal") {
			c.ctx.Notification.AddErrorMessage("Invalid output mode. Valid options: true, 256, normal")
			return nil
		}
	}

	// Update the configuration through GuiCommon
	// Note: This assumes the implementation will provide a way to update config
	// For now, we'll need to add this functionality to the GuiCommon interface
	config := c.ctx.GuiCommon.GetConfig()
	gui := c.ctx.GuiCommon.GetGui()

	switch setting {
	case "cursor":
		config.ShowCursor = value == "true" || value == "on" || value == "yes"
		gui.Cursor = config.ShowCursor
	case "markdown":
		config.MarkdownRendering = value == "true" || value == "on" || value == "yes"
	case "theme":
		// Validate theme exists by checking against available theme names
		themeNames := presentation.GetThemeNames()
		themeExists := false
		for _, name := range themeNames {
			if name == value {
				themeExists = true
				break
			}
		}
		if themeExists {
			oldTheme := config.Theme
			config.Theme = value
			// Emit theme changed event for components to react
			c.ctx.CommandEventBus.Emit("theme.changed", map[string]interface{}{
				"oldTheme": oldTheme,
				"newTheme": value,
				"config":   config,
			})
		}
	case "markdowntheme", "markdown-theme":
		// Validate the glamour theme
		availableThemes := presentation.GetAllAvailableGlamourStyles()
		validTheme := value == "auto"
		for _, theme := range availableThemes {
			if theme == value {
				validTheme = true
				break
			}
		}
		if validTheme {
			config.GlamourTheme = value
			c.ctx.Notification.AddSystemMessage(fmt.Sprintf("Markdown theme updated to %s", value))
		} else {
			c.ctx.Notification.AddErrorMessage(fmt.Sprintf("Invalid markdown theme. Available: %s, auto", strings.Join(availableThemes, ", ")))
			return nil
		}
	case "wrap":
		config.WrapMessages = value == "true" || value == "on" || value == "yes"
	case "timestamps":
		config.ShowTimestamps = value == "true" || value == "on" || value == "yes"
	case "output", "outputmode":
		config.OutputMode = value
		c.ctx.Notification.AddSystemMessage("Output mode updated. Restart the application for changes to take effect.")
	case "messagesborder", "messages-border", "border":
		config.ShowMessagesBorder = value == "true" || value == "on" || value == "yes"
		c.ctx.Notification.AddSystemMessage("Border setting updated. Please restart the application for changes to take effect.")
	case "userlabel", "user-label":
		config.UserLabel = value
	case "assistantlabel", "assistant-label":
		config.AssistantLabel = value
	case "systemlabel", "system-label":
		config.SystemLabel = value
	case "errorlabel", "error-label":
		config.ErrorLabel = value
	}

	// Save config
	if err := c.ctx.ConfigManager.Save(config); err != nil {
		c.ctx.Logger.Debug(fmt.Sprintf("Config save failed: %v", err))
	}

	// Don't show generic message for settings that have custom messages
	switch setting {
	case "messagesborder", "messages-border", "border", "output", "outputmode", "output-mode", "markdowntheme", "markdown-theme":
		// These settings have their own custom messages or error handling
	default:
		c.ctx.Notification.AddSystemMessage(fmt.Sprintf("Updated %s to %s", setting, value))
	}

	return nil
}

func (c *ConfigCommand) resetConfig() error {
	// Get default config
	defaultConfig := c.ctx.ConfigManager.GetDefaultConfig()

	// Save the default config
	if err := c.ctx.ConfigManager.Save(defaultConfig); err != nil {
		c.ctx.Notification.AddErrorMessage(fmt.Sprintf("Failed to reset config: %v", err))
		return nil
	}

	// Emit theme changed event for components to react
	c.ctx.CommandEventBus.Emit("theme.changed", map[string]interface{}{
		"oldTheme": "unknown",
		"newTheme": defaultConfig.Theme,
		"config":   defaultConfig,
	})

	c.ctx.Notification.AddSystemMessage("Configuration reset to defaults. Some changes may require restarting the application.")

	return nil
}
