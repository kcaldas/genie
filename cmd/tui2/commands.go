package tui2

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

func (app *App) cmdHelp(args []string) error {
	var content strings.Builder
	
	// If specific command requested, show detailed help for that command
	if len(args) > 0 {
		cmdName := args[0]
		if cmd := app.commandHandler.GetCommand(cmdName); cmd != nil {
			content.WriteString(fmt.Sprintf("Command: /%s\n", cmd.Name))
			content.WriteString(fmt.Sprintf("Description: %s\n", cmd.Description))
			content.WriteString(fmt.Sprintf("Usage: %s\n", cmd.Usage))
			
			if len(cmd.Aliases) > 0 {
				content.WriteString(fmt.Sprintf("Aliases: %s\n", strings.Join(cmd.Aliases, ", ")))
			}
			
			if len(cmd.Examples) > 0 {
				content.WriteString("\nExamples:\n")
				for _, example := range cmd.Examples {
					content.WriteString(fmt.Sprintf("  %s\n", example))
				}
			}
		} else {
			content.WriteString(fmt.Sprintf("Unknown command: /%s\n", cmdName))
			content.WriteString("Use /help to see all available commands.\n")
		}
	} else {
		// Show categorized help for all commands
		content.WriteString("Available commands:\n\n")
		
		registry := app.commandHandler.GetRegistry()
		commandsByCategory := registry.GetCommandsByCategory()
		
		// Define category order for better organization
		categoryOrder := []string{"General", "Chat", "Configuration", "Navigation", "Layout", "Debug"}
		
		for _, category := range categoryOrder {
			if commands, exists := commandsByCategory[category]; exists {
				content.WriteString(fmt.Sprintf("## %s\n", category))
				
				for _, cmd := range commands {
					content.WriteString(fmt.Sprintf("  /%s", cmd.Name))
					
					// Add aliases in parentheses
					if len(cmd.Aliases) > 0 {
						aliasStr := strings.Join(cmd.Aliases, ", ")
						content.WriteString(fmt.Sprintf(" (%s)", aliasStr))
					}
					
					content.WriteString(fmt.Sprintf(" - %s\n", cmd.Description))
				}
				content.WriteString("\n")
			}
		}
		
		// Add any remaining categories not in the predefined order
		for category, commands := range commandsByCategory {
			found := false
			for _, orderedCategory := range categoryOrder {
				if category == orderedCategory {
					found = true
					break
				}
			}
			
			if !found && len(commands) > 0 {
				content.WriteString(fmt.Sprintf("## %s\n", category))
				for _, cmd := range commands {
					content.WriteString(fmt.Sprintf("  /%s", cmd.Name))
					if len(cmd.Aliases) > 0 {
						aliasStr := strings.Join(cmd.Aliases, ", ")
						content.WriteString(fmt.Sprintf(" (%s)", aliasStr))
					}
					content.WriteString(fmt.Sprintf(" - %s\n", cmd.Description))
				}
				content.WriteString("\n")
			}
		}
		
		content.WriteString("Use /help <command> for detailed information about a specific command.\n\n")
		
		content.WriteString("Keyboard shortcuts:\n")
		content.WriteString("  Tab - Switch between panels\n")
		content.WriteString("  Ctrl+C - Exit application\n")
		content.WriteString("  Arrow keys - Navigate in panels\n")
		content.WriteString("  y - Copy selected message (in messages panel)\n")
		content.WriteString("  Y - Copy all messages (in messages panel)\n")
	}

	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: content.String(),
	})

	return app.refreshUI()
}

func (app *App) cmdClear(args []string) error {
	app.chatController.ClearConversation()
	return app.refreshUI()
}

func (app *App) cmdDebug(args []string) error {
	app.debugComponent.ToggleVisibility()
	app.gui.Update(func(g *gocui.Gui) error {
		return nil
	})
	return nil
}

func (app *App) cmdConfig(args []string) error {
	if len(args) == 0 {
		return app.showConfigMenu()
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: /config <setting> <value>")
	}

	setting := args[0]
	value := strings.Join(args[1:], " ")

	return app.updateConfig(setting, value)
}

func (app *App) cmdExit(args []string) error {
	return gocui.ErrQuit
}

func (app *App) cmdTheme(args []string) error {
	if len(args) == 0 {
		themes := presentation.GetThemeNames()
		content := fmt.Sprintf("Available themes: %s\nCurrent theme: %s",
			strings.Join(themes, ", "),
			app.uiState.GetConfig().Theme)

		app.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: content,
		})
		return app.refreshUI()
	}

	themeName := args[0]
	return app.updateConfig("theme", themeName)
}

func (app *App) showConfigMenu() error {
	config := app.uiState.GetConfig()

	content := fmt.Sprintf(`Current configuration:
  Show cursor: %v
  Markdown rendering: %v
  Theme: %s
  Wrap messages: %v
  Show timestamps: %v

Use /config <setting> <value> to change settings.
Available settings: cursor, markdown, theme, wrap, timestamps`,
		config.ShowCursor,
		config.MarkdownRendering,
		config.Theme,
		config.WrapMessages,
		config.ShowTimestamps,
	)

	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: content,
	})

	return app.refreshUI()
}

func (app *App) updateConfig(setting, value string) error {
	app.uiState.UpdateConfig(func(config *types.Config) {
		switch setting {
		case "cursor":
			config.ShowCursor = value == "true" || value == "on" || value == "yes"
			app.gui.Cursor = config.ShowCursor
		case "markdown":
			config.MarkdownRendering = value == "true" || value == "on" || value == "yes"
		case "theme":
			if theme := presentation.GetTheme(value); theme != nil {
				config.Theme = value
				app.messageFormatter, _ = presentation.NewMessageFormatter(config, theme)
			}
		case "wrap":
			config.WrapMessages = value == "true" || value == "on" || value == "yes"
		case "timestamps":
			config.ShowTimestamps = value == "true" || value == "on" || value == "yes"
		}
	})

	if err := app.helpers.Config.Save(app.uiState.GetConfig()); err != nil {
		app.stateAccessor.AddDebugMessage(fmt.Sprintf("Failed to save config: %v", err))
	}

	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: fmt.Sprintf("Updated %s to %s", setting, value),
	})

	return app.refreshUI()
}

func (app *App) cmdFocus(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: /focus <panel>")
	}

	panelName := args[0]
	return app.setCurrentView(panelName)
}

func (app *App) cmdToggle(args []string) error {
	// Toggle command removed since screenManager is removed
	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: "Toggle command is no longer available",
	})
	return app.refreshUI()
}

func (app *App) cmdLayout(args []string) error {
	// Layout command simplified since screenManager is removed
	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: "Layout uses simple 5-panel system. Use /focus to switch between panels.",
	})
	return app.refreshUI()
}

func (app *App) refreshUI() error {
	app.gui.Update(func(g *gocui.Gui) error {
		if err := app.messagesComponent.Render(); err != nil {
			return err
		}
		if app.debugComponent.IsVisible() {
			return app.debugComponent.Render()
		}
		return nil
	})
	return nil
}

