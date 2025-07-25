package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/logging"
)

type ConfigCommand struct {
	BaseCommand
	configManager   *helpers.ConfigManager
	commandEventBus *events.CommandEventBus
	guiCommon       types.Gui
	notification    *controllers.ChatController
}

func NewConfigCommand(configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, guiCommon types.Gui, notification *controllers.ChatController) *ConfigCommand {
	return &ConfigCommand{
		BaseCommand: BaseCommand{
			Name:        "config",
			Description: "Configure TUI settings (cursor, markdown, theme, diff-theme, wrap, timestamps, output, mouse, vim, tools). Use --global to save to global config (~/.genie), otherwise saves to local config (.genie).",
			Usage:       ":config [--global] <setting> <value> | :config [--global] tool <name> <property> <value> | :config [--global] reset",
			Examples: []string{
				":config",
				":config theme dark",
				":config --global theme dark",
				":config markdown-theme dracula",
				":config markdown-theme auto",
				":config diff-theme github",
				":config diff-theme auto",
				":config cursor true",
				":config markdown false",
				":config mouse true",
				":config mouse false",
				":config output true",
				":config output 256",
				":config output normal",
				":config border false",
				":config messagesborder true",
				":config userlabel >",
				":config assistantlabel ★",
				":config systemlabel ■",
				":config errorlabel ✗",
				":config tool bash accept true",
				":config --global tool TodoWrite hide true",
				":config reset",
				":config --global reset",
			},
			Aliases:  []string{"cfg", "settings"},
			Category: "Configuration",
		},
		configManager:   configManager,
		commandEventBus: commandEventBus,
		guiCommon:       guiCommon,
		notification:    notification,
	}
}

// logger returns the current global logger (updated dynamically when debug is toggled)
func (c *ConfigCommand) logger() logging.Logger {
	return logging.GetGlobalLogger()
}

func (c *ConfigCommand) Execute(args []string) error {
	if len(args) == 0 {
		c.commandEventBus.Emit("user.input.command", ":help")
		return nil
	}

	// Check for --global flag
	global := false
	filteredArgs := []string{}
	for _, arg := range args {
		if arg == "--global" {
			global = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Handle reset command
	if len(filteredArgs) > 0 && filteredArgs[0] == "reset" {
		return c.resetConfig(global)
	}

	// Handle tool configuration: :config tool {toolName} {property} {value}
	if len(filteredArgs) >= 1 && filteredArgs[0] == "tool" && len(filteredArgs) >= 4 {
		toolName := filteredArgs[1]
		property := filteredArgs[2]
		value := filteredArgs[3]
		return c.updateToolConfig(toolName, property, value, global)
	}

	if len(filteredArgs) < 2 {
		globalFlag := ""
		if global {
			globalFlag = " --global"
		}
		return fmt.Errorf("usage: :config%s <setting> <value> or :config%s reset", globalFlag, globalFlag)
	}

	setting := filteredArgs[0]
	value := strings.Join(filteredArgs[1:], " ")

	return c.updateConfig(setting, value, global)
}

func (c *ConfigCommand) updateConfig(setting, value string, global bool) error {
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
		if value == "true" || value == "on" || value == "yes" || value == "enabled" {
			config.ShowCursor = "enabled"
		} else {
			config.ShowCursor = "disabled"
		}
		gui.Cursor = config.IsShowCursorEnabled()
	case "markdown":
		if value == "true" || value == "on" || value == "yes" || value == "enabled" {
			config.MarkdownRendering = "enabled"
		} else {
			config.MarkdownRendering = "disabled"
		}
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
		if value == "true" || value == "on" || value == "yes" || value == "enabled" {
			config.WrapMessages = "enabled"
		} else {
			config.WrapMessages = "disabled"
		}
	case "timestamps":
		config.ShowTimestamps = value == "true" || value == "on" || value == "yes"
	case "output", "outputmode":
		config.OutputMode = value
		c.notification.AddSystemMessage("Output mode updated. Restart the application for changes to take effect.")
	case "messagesborder", "messages-border", "border":
		if value == "true" || value == "on" || value == "yes" || value == "enabled" {
			config.ShowMessagesBorder = "enabled"
		} else {
			config.ShowMessagesBorder = "disabled"
		}
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
	case "mouse":
		if value == "true" || value == "on" || value == "yes" || value == "enabled" {
			config.EnableMouse = "enabled"
		} else {
			config.EnableMouse = "disabled"
		}
		gui := c.guiCommon.GetGui()
		gui.Mouse = config.IsMouseEnabled()
		if config.IsMouseEnabled() {
			c.notification.AddSystemMessage("Mouse support enabled (gocui mouse interactions). Terminal text selection disabled.")
		} else {
			c.notification.AddSystemMessage("Mouse support disabled. Terminal native text selection enabled.")
		}
	}

	// Save config
	if err := c.configManager.SaveWithScope(config, global); err != nil {
		c.logger().Debug("Config save failed", "error", err)
	}

	// Don't show generic message for settings that have custom messages
	scope := "local"
	if global {
		scope = "global"
	}
	switch setting {
	case "messagesborder", "messages-border", "border", "output", "outputmode", "output-mode", "markdowntheme", "markdown-theme", "difftheme", "diff-theme":
		// These settings have their own custom messages or error handling
	default:
		c.notification.AddSystemMessage(fmt.Sprintf("Updated %s to %s (%s config)", setting, value, scope))
	}

	return nil
}

func (c *ConfigCommand) updateToolConfig(toolName, property, value string, global bool) error {
	// Validate property
	if property != "accept" && property != "hide" {
		return fmt.Errorf("invalid tool property '%s'. Valid options: accept, hide", property)
	}

	// Validate boolean value
	boolValue := value == "true" || value == "on" || value == "yes"
	if value != "true" && value != "false" && value != "on" && value != "off" && value != "yes" && value != "no" {
		return fmt.Errorf("invalid value '%s'. Valid options: true/false, on/off, yes/no", value)
	}

	// Update the configuration through ConfigManager
	config := c.configManager.GetConfig()
	
	// Initialize ToolConfigs map if nil
	if config.ToolConfigs == nil {
		config.ToolConfigs = make(map[string]types.ToolConfig)
	}

	// Get or create tool config
	toolConfig := config.ToolConfigs[toolName]

	// Update the specific property
	switch property {
	case "accept":
		toolConfig.AutoAccept = boolValue
	case "hide":
		toolConfig.Hide = boolValue
	}

	// Save back to map
	config.ToolConfigs[toolName] = toolConfig
	
	// Save to specified scope
	err := c.configManager.SaveWithScope(config, global)

	if err != nil {
		c.logger().Debug("Tool config save failed", "error", err)
		return err
	}

	scope := "local"
	if global {
		scope = "global"
	}
	c.notification.AddSystemMessage(fmt.Sprintf("Updated tool '%s' %s to %v (%s config)", toolName, property, boolValue, scope))
	return nil
}

func (c *ConfigCommand) resetConfig(global bool) error {
	if global {
		// For global reset, save defaults to global config
		defaultConfig := c.configManager.GetDefaultConfig()
		if err := c.configManager.SaveWithScope(defaultConfig, true); err != nil {
			c.notification.AddErrorMessage(fmt.Sprintf("Failed to reset global config: %v", err))
			return nil
		}
		
		// Emit theme changed event for components to react
		c.commandEventBus.Emit("theme.changed", map[string]interface{}{
			"oldTheme": "unknown",
			"newTheme": defaultConfig.Theme,
			"config":   defaultConfig,
		})
		
		c.notification.AddSystemMessage("Global configuration reset to defaults. Some changes may require restarting the application.")
	} else {
		// For local reset, delete the local config file to allow global config to take precedence
		if err := c.configManager.DeleteLocalConfig(); err != nil {
			c.notification.AddErrorMessage(fmt.Sprintf("Failed to reset local config: %v", err))
			return nil
		}
		
		// Reload config to reflect the change (global + defaults will now apply)
		if err := c.configManager.Reload(); err != nil {
			c.logger().Debug("Failed to reload config after local reset", "error", err)
		}
		
		// Get the new effective config for theme event
		newConfig := c.configManager.GetConfig()
		
		// Emit theme changed event for components to react
		c.commandEventBus.Emit("theme.changed", map[string]interface{}{
			"oldTheme": "unknown",
			"newTheme": newConfig.Theme,
			"config":   newConfig,
		})
		
		c.notification.AddSystemMessage("Local configuration removed. Now using global/default settings. Some changes may require restarting the application.")
	}

	return nil
}
