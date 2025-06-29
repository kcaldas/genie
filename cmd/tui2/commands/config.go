package commands

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/types"
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
			Usage:       ":config <setting> <value>",
			Examples: []string{
				":config",
				":config theme dark",
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
			},
			Aliases:  []string{"cfg", "settings"},
			Category: "Configuration",
		},
		ctx: ctx,
	}
}

func (c *ConfigCommand) Execute(args []string) error {
	if len(args) == 0 {
		return c.ctx.ShowHelpDialog("Configuration")
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: :config <setting> <value>")
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
		if theme := presentation.GetTheme(value); theme != nil {
			config.Theme = value
			// Note: Message formatter update would need to be handled by the app
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
		c.ctx.StateAccessor.AddDebugMessage(fmt.Sprintf("Failed to save config: %v", err))
	}

	// Don't show generic message for settings that have custom messages
	switch setting {
	case "messagesborder", "messages-border", "border", "output", "outputmode", "output-mode":
		// These settings have their own custom messages
	default:
		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: fmt.Sprintf("Updated %s to %s", setting, value),
		})
	}

	return c.ctx.RefreshUI()
}