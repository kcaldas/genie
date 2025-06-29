package tui2

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

func (app *App) cmdHelp(args []string) error {
	// Determine category to show (if any)
	category := ""
	if len(args) > 0 {
		category = args[0]
	}
	
	// Show help dialog instead of adding to chat
	return app.showHelpDialog(category)
}

func (app *App) cmdClear(args []string) error {
	app.chatController.ClearConversation()
	return app.refreshUI()
}

func (app *App) cmdDebug(args []string) error {
	app.debugComponent.ToggleVisibility()
	
	// Force layout refresh and cleanup when hiding
	app.gui.Update(func(g *gocui.Gui) error {
		if !app.debugComponent.IsVisible() {
			// Delete the debug view when hiding
			g.DeleteView("debug")
		}
		return nil
	})
	
	return nil
}

func (app *App) cmdConfig(args []string) error {
	if len(args) == 0 {
		return app.showHelpDialog("Configuration")
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

