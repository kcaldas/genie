package tui2

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/layout"
	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

func (app *App) cmdHelp(args []string) error {
	help := app.commandHandler.GetCommandHelp()
	
	var content strings.Builder
	content.WriteString("Available commands:\n\n")
	
	for cmd, desc := range help {
		content.WriteString(fmt.Sprintf("  /%s - %s\n", cmd, desc))
	}
	
	content.WriteString("\nKeyboard shortcuts:\n")
	content.WriteString("  Tab - Switch between panels\n")
	content.WriteString("  Ctrl+C - Exit application\n")
	content.WriteString("  Arrow keys - Navigate in panels\n")
	content.WriteString("  y - Copy selected message (in messages panel)\n")
	content.WriteString("  Y - Copy all messages (in messages panel)\n")
	
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
	app.debugCtx.ToggleVisibility()
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
	app.layoutManager.ToggleScreenMode()
	return nil
}

func (app *App) cmdLayout(args []string) error {
	if len(args) == 0 {
		screenMode := app.layoutManager.GetScreenManager().GetMode()
		modeStr := "normal"
		switch screenMode {
		case layout.SCREEN_HALF:
			modeStr = "half"
		case layout.SCREEN_FULL:
			modeStr = "full"
		}
		
		app.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: fmt.Sprintf("Current layout mode: %s\nUse /layout <mode> to change (normal, half, full)", modeStr),
		})
		return app.refreshUI()
	}
	
	mode := args[0]
	switch mode {
	case "normal":
		app.layoutManager.GetScreenManager().SetMode(layout.SCREEN_NORMAL)
	case "half":
		app.layoutManager.GetScreenManager().SetMode(layout.SCREEN_HALF)
	case "full":
		app.layoutManager.GetScreenManager().SetMode(layout.SCREEN_FULL)
	default:
		return fmt.Errorf("unknown layout mode: %s", mode)
	}
	
	return nil
}

func (app *App) refreshUI() error {
	app.gui.Update(func(g *gocui.Gui) error {
		if err := app.messagesCtx.Render(); err != nil {
			return err
		}
		if app.debugCtx.IsVisible() {
			return app.debugCtx.Render()
		}
		return nil
	})
	return nil
}