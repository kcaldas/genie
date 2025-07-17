package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type ConfigCommand struct {
	BaseCommand
	configManager   *helpers.ConfigManager
	commandEventBus *events.CommandEventBus
	guiCommon       types.Gui
	notification    *controllers.ChatController
	logger          types.Logger
}

func NewConfigCommand(configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, guiCommon types.Gui, notification *controllers.ChatController, logger types.Logger) *ConfigCommand {
	return &ConfigCommand{
		BaseCommand: BaseCommand{
			Name:        "config",
			Description: "Configure TUI settings (cursor, markdown, theme, diff-theme, wrap, timestamps, output)",
			Usage:       ":config <setting> <value> | :config reset",
			Examples: []string{
				":config",
				":config theme dark",
				":config markdown-theme dracula",
				":config markdown-theme auto",
				":config diff-theme github",
				":config diff-theme auto",
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
		configManager:   configManager,
		commandEventBus: commandEventBus,
		guiCommon:       guiCommon,
		notification:    notification,
		logger:          logger,
	}
}

func (c *ConfigCommand) Execute(args []string) error {
	if len(args) == 0 {
		c.commandEventBus.Emit("user.input.command", ":help")
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
			c.notification.AddErrorMessage("Invalid output mode. Valid options: true, 256, normal")
			return nil
		}
	}

	// Update the configuration through ConfigManager
	config := c.configManager.GetConfig()
	gui := c.guiCommon.GetGui()

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
			c.commandEventBus.Emit("theme.changed", map[string]interface{}{
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
			c.notification.AddSystemMessage(fmt.Sprintf("Markdown theme updated to %s", value))
		} else {
			c.notification.AddErrorMessage(fmt.Sprintf("Invalid markdown theme. Available: %s, auto", strings.Join(availableThemes, ", ")))
			return nil
		}
	case "difftheme", "diff-theme":
		// Validate the diff theme
		availableThemes := presentation.GetDiffThemeNames()
		validTheme := value == "auto"
		for _, theme := range availableThemes {
			if theme == value {
				validTheme = true
				break
			}
		}
		if validTheme {
			config.DiffTheme = value
			c.notification.AddSystemMessage(fmt.Sprintf("Diff theme updated to %s", value))
		} else {
			c.notification.AddErrorMessage(fmt.Sprintf("Invalid diff theme. Available: %s, auto", strings.Join(availableThemes, ", ")))
			return nil
		}
	case "wrap":
		config.WrapMessages = value == "true" || value == "on" || value == "yes"
	case "timestamps":
		config.ShowTimestamps = value == "true" || value == "on" || value == "yes"
	case "output", "outputmode":
		config.OutputMode = value
		c.notification.AddSystemMessage("Output mode updated. Restart the application for changes to take effect.")
	case "messagesborder", "messages-border", "border":
		config.ShowMessagesBorder = value == "true" || value == "on" || value == "yes"
		c.notification.AddSystemMessage("Border setting updated. Please restart the application for changes to take effect.")
	case "userlabel", "user-label":
		config.UserLabel = value
	case "assistantlabel", "assistant-label":
		config.AssistantLabel = value
	case "systemlabel", "system-label":
		config.SystemLabel = value
	case "errorlabel", "error-label":
		config.ErrorLabel = value
	case "vimmode", "vim-mode", "vim":
		config.VimMode = value == "true" || value == "on" || value == "yes"
		c.notification.AddSystemMessage("Vim mode updated.")
		// Emit event to refresh keybindings
		c.commandEventBus.Emit("vim.mode.changed", config.VimMode)
	}

	// Save config
	if err := c.configManager.Save(config); err != nil {
		c.logger.Debug(fmt.Sprintf("Config save failed: %v", err))
	}

	// Don't show generic message for settings that have custom messages
	switch setting {
	case "messagesborder", "messages-border", "border", "output", "outputmode", "output-mode", "markdowntheme", "markdown-theme", "difftheme", "diff-theme":
		// These settings have their own custom messages or error handling
	default:
		c.notification.AddSystemMessage(fmt.Sprintf("Updated %s to %s", setting, value))
	}

	return nil
}

func (c *ConfigCommand) resetConfig() error {
	// Get default config
	defaultConfig := c.configManager.GetDefaultConfig()

	// Save the default config
	if err := c.configManager.Save(defaultConfig); err != nil {
		c.notification.AddErrorMessage(fmt.Sprintf("Failed to reset config: %v", err))
		return nil
	}

	// Emit theme changed event for components to react
	c.commandEventBus.Emit("theme.changed", map[string]interface{}{
		"oldTheme": "unknown",
		"newTheme": defaultConfig.Theme,
		"config":   defaultConfig,
	})

	c.notification.AddSystemMessage("Configuration reset to defaults. Some changes may require restarting the application.")

	return nil
}
