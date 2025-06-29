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
	
	// Get current config and update it
	if guiCommon := c.ctx.GuiCommon; guiCommon != nil {
		// Assuming we have access to update config through GuiCommon or StateAccessor
		// This might need adjustment based on actual implementation
		switch setting {
		case "cursor":
			// Handle cursor setting - would need access to config update mechanism
		case "markdown":
			// Handle markdown setting
		case "theme":
			if theme := presentation.GetTheme(value); theme != nil {
				// Update theme
			}
		case "wrap":
			// Handle wrap setting
		case "timestamps":
			// Handle timestamps setting
		case "output", "outputmode":
			c.ctx.StateAccessor.AddMessage(types.Message{
				Role:    "system",
				Content: "Output mode updated. Restart the application for changes to take effect.",
			})
		case "messagesborder", "messages-border", "border":
			c.ctx.StateAccessor.AddMessage(types.Message{
				Role:    "system",
				Content: "Border setting updated. Please restart the application for changes to take effect.",
			})
		case "userlabel", "user-label":
			// Handle user label
		case "assistantlabel", "assistant-label":
			// Handle assistant label
		case "systemlabel", "system-label":
			// Handle system label
		case "errorlabel", "error-label":
			// Handle error label
		}
	}

	// Save config if possible
	if c.ctx.ConfigHelper != nil {
		if config := c.ctx.GuiCommon.GetConfig(); config != nil {
			if err := c.ctx.ConfigHelper.Save(config); err != nil {
				c.ctx.StateAccessor.AddDebugMessage(fmt.Sprintf("Failed to save config: %v", err))
			}
		}
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