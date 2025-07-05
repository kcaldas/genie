package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
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
		return c.ctx.ShowHelpInTextViewer()
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
			c.ctx.StateAccessor.AddMessage(types.Message{
				Role:    "error",
				Content: "Invalid output mode. Valid options: true, 256, normal",
			})
			return c.ctx.RefreshUI()
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
			config.Theme = value
			// Note: Message formatter update would need to be handled by the app
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
			c.ctx.StateAccessor.AddMessage(types.Message{
				Role:    "system",
				Content: fmt.Sprintf("Markdown theme updated to %s", value),
			})
		} else {
			c.ctx.StateAccessor.AddMessage(types.Message{
				Role:    "error",
				Content: fmt.Sprintf("Invalid markdown theme. Available: %s, auto", strings.Join(availableThemes, ", ")),
			})
			return c.ctx.RefreshUI()
		}
	case "wrap":
		config.WrapMessages = value == "true" || value == "on" || value == "yes"
	case "timestamps":
		config.ShowTimestamps = value == "true" || value == "on" || value == "yes"
	case "output", "outputmode":
		config.OutputMode = value
		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: "Output mode updated. Restart the application for changes to take effect.",
		})
	case "messagesborder", "messages-border", "border":
		config.ShowMessagesBorder = value == "true" || value == "on" || value == "yes"
		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: "Border setting updated. Please restart the application for changes to take effect.",
		})
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
	if err := c.ctx.ConfigHelper.Save(config); err != nil {
		c.ctx.StateAccessor.AddDebugMessage(fmt.Sprintf("Config save failed: %v", err))
	}

	// Don't show generic message for settings that have custom messages
	switch setting {
	case "messagesborder", "messages-border", "border", "output", "outputmode", "output-mode", "markdowntheme", "markdown-theme":
		// These settings have their own custom messages or error handling
	default:
		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: fmt.Sprintf("Updated %s to %s", setting, value),
		})
	}

	return c.ctx.RefreshUI()
}

func (c *ConfigCommand) resetConfig() error {
	// Get default config
	defaultConfig := c.ctx.ConfigHelper.GetDefaultConfig()
	
	// Save the default config
	if err := c.ctx.ConfigHelper.Save(defaultConfig); err != nil {
		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Failed to reset config: %v", err),
		})
		return c.ctx.RefreshUI()
	}

	// Apply theme changes to the running application
	if err := c.ctx.RefreshTheme(); err != nil {
		c.ctx.StateAccessor.AddDebugMessage(fmt.Sprintf("Theme refresh failed after reset: %v", err))
	}

	c.ctx.StateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: "Configuration reset to defaults. Some changes may require restarting the application.",
	})

	return c.ctx.RefreshUI()
}