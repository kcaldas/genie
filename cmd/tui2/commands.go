package tui2

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/charmbracelet/glamour"
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
		currentTheme := app.uiState.GetConfig().Theme
		glamourStyle := presentation.GetGlamourStyleForTheme(currentTheme)
		
		content := fmt.Sprintf("Available themes: %s\n\nCurrent theme: %s\nMarkdown style: %s\n\nUsage: /theme <name>",
			strings.Join(themes, ", "),
			currentTheme,
			glamourStyle)

		app.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: content,
		})
		return app.refreshUI()
	}

	themeName := args[0]
	
	// Update config through the existing method
	err := app.updateConfig("theme", themeName)
	if err != nil {
		return err
	}
	
	// Refresh component theme colors
	app.refreshComponentThemes()
	
	return nil
}


func (app *App) updateConfig(setting, value string) error {
	// Validate output mode before updating config
	if setting == "output" || setting == "outputmode" {
		if !(value == "true" || value == "256" || value == "normal") {
			app.stateAccessor.AddMessage(types.Message{
				Role:    "error",
				Content: "Invalid output mode. Valid options: true, 256, normal",
			})
			return app.refreshUI()
		}
	}
	
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
				// Update message formatter with new theme and glamour style
				app.messageFormatter, _ = presentation.NewMessageFormatter(config, theme)
			}
		case "wrap":
			config.WrapMessages = value == "true" || value == "on" || value == "yes"
		case "timestamps":
			config.ShowTimestamps = value == "true" || value == "on" || value == "yes"
		case "output", "outputmode":
			config.OutputMode = value
			app.stateAccessor.AddMessage(types.Message{
				Role:    "system",
				Content: "Output mode updated. Restart the application for changes to take effect.",
			})
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
		if err := app.inputComponent.Render(); err != nil {
			return err
		}
		if app.debugComponent.IsVisible() {
			if err := app.debugComponent.Render(); err != nil {
				return err
			}
		}
		if err := app.statusComponent.Render(); err != nil {
			return err
		}
		return nil
	})
	return nil
}

func (app *App) cmdMarkdownDemo(args []string) error {
	sampleMarkdown := `# Theme Demo

This is **bold text** and *italic text*.

## Code Block
` + "```go" + `
func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

## List
- Item 1
- Item 2
  - Nested item
- Item 3

> This is a blockquote with **emphasis**.

[Link example](https://example.com)

---

Current theme integrates both TUI colors and markdown rendering!`

	app.stateAccessor.AddMessage(types.Message{
		Role:    "assistant",
		Content: sampleMarkdown,
	})
	return app.refreshUI()
}

func (app *App) cmdGlamourTest(args []string) error {
	if len(args) == 0 {
		// Show all available glamour styles
		styles := presentation.GetAllAvailableGlamourStyles()
		content := "Available glamour styles:\n"
		for _, style := range styles {
			content += "- " + style + "\n"
		}
		content += "\nUsage: /glamour-test <style>"
		
		app.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: content,
		})
		return app.refreshUI()
	}

	styleName := args[0]
	
	// Test the glamour style with a sample
	sampleMarkdown := `# Glamour Style: ` + styleName + `

Testing **` + styleName + `** glamour theme:

## Features
- **Bold text**
- *Italic text*  
- ~~Strikethrough~~

` + "```go" + `
func glamourTest() {
    fmt.Println("Testing: ` + styleName + `")
}
` + "```" + `

> Blockquote with **emphasis** in ` + styleName + ` style.

### List
1. First item
2. Second item
   - Nested item

---

Style: **` + styleName + `**`

	// Create a temporary renderer with the specified style
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(styleName),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return fmt.Errorf("invalid glamour style: %s", styleName)
	}
	
	// Render the sample
	rendered, err := renderer.Render(sampleMarkdown)
	if err != nil {
		return fmt.Errorf("failed to render with style %s: %v", styleName, err)
	}

	app.stateAccessor.AddMessage(types.Message{
		Role:    "assistant",
		Content: rendered, // Already rendered, so don't re-process as markdown
	})
	return app.refreshUI()
}


func (app *App) refreshComponentThemes() {
	// Refresh border colors for all components
	app.messagesComponent.RefreshThemeColors()
	app.inputComponent.RefreshThemeColors()
	app.debugComponent.RefreshThemeColors()
	app.statusComponent.RefreshThemeColors()
}

