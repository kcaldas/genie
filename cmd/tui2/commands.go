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
		return fmt.Errorf("usage: :config <setting> <value>")
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
		
		content := fmt.Sprintf("Available themes: %s\n\nCurrent theme: %s\nMarkdown style: %s\n\nUsage: :theme <name>",
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
		case "messagesborder", "messages-border", "border":
			config.ShowMessagesBorder = value == "true" || value == "on" || value == "yes"
			app.stateAccessor.AddMessage(types.Message{
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
	})

	if err := app.helpers.Config.Save(app.uiState.GetConfig()); err != nil {
		app.stateAccessor.AddDebugMessage(fmt.Sprintf("Failed to save config: %v", err))
	}

	// Don't show generic message for settings that have custom messages
	switch setting {
	case "messagesborder", "messages-border", "border", "output", "outputmode", "output-mode":
		// These settings have their own custom messages
	default:
		app.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: fmt.Sprintf("Updated %s to %s", setting, value),
		})
	}

	return app.refreshUI()
}

func (app *App) cmdFocus(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: :focus <panel>")
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
		Content: "Layout uses simple 5-panel system. Use :focus to switch between panels.",
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
		content += "\nUsage: :glamour-test <style>"
		
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
	// Update global GUI frame colors
	config := app.uiState.GetConfig()
	theme := presentation.GetTheme(config.Theme)
	if theme != nil {
		app.gui.FrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderDefault)
		app.gui.SelFrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderFocused)
	}
	
	// Refresh border colors for all components
	app.messagesComponent.RefreshThemeColors()
	app.inputComponent.RefreshThemeColors()
	app.debugComponent.RefreshThemeColors()
	app.statusComponent.RefreshThemeColors()
}

func (app *App) cmdYank(args []string) error {
	// Parse vim-style yank command: :y[count][direction]
	// Examples: :y, :y3, :y2k, :y5j
	
	count := 1
	direction := "k" // default to up (k = previous messages)
	
	if len(args) > 0 {
		arg := args[0]
		// Parse count and direction from argument like "2k", "3j", "5"
		parsedCount, parsedDirection := app.parseYankArgument(arg)
		if parsedCount > 0 {
			count = parsedCount
		}
		if parsedDirection != "" {
			direction = parsedDirection
		}
	}
	
	var messages []types.Message
	var description string
	
	switch direction {
	case "k", "": // up/previous messages (default)
		messages = app.stateAccessor.GetLastMessages(count)
		if count == 1 {
			description = "last message"
		} else {
			description = fmt.Sprintf("last %d messages", count)
		}
	case "j": // down/next messages (not very useful in chat context, but for completeness)
		// For now, just treat as same as k since we don't have cursor position
		messages = app.stateAccessor.GetLastMessages(count)
		description = fmt.Sprintf("last %d messages", count)
	case "-": // relative positioning: copy the Nth message from the end
		totalMessages := app.stateAccessor.GetMessageCount()
		if count > totalMessages {
			messages = []types.Message{}
		} else {
			// Get a single message at relative position
			// count=1 means last message, count=2 means 2nd to last, etc.
			start := totalMessages - count
			messages = app.stateAccessor.GetMessageRange(start, 1)
		}
		if count == 1 {
			description = "last message"
		} else {
			description = fmt.Sprintf("message %d from end", count)
		}
	default:
		return fmt.Errorf("unknown direction: %s (use k for up, j for down, - for relative)", direction)
	}
	
	if len(messages) == 0 {
		app.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: "No messages to copy.",
		})
		return app.refreshUI()
	}
	
	// Format messages for clipboard
	var content strings.Builder
	for i, msg := range messages {
		if i > 0 {
			content.WriteString("\n---\n\n")
		}
		content.WriteString(fmt.Sprintf("[%s] %s", strings.ToUpper(msg.Role), msg.Content))
	}
	
	// Copy to clipboard
	if err := app.helpers.Clipboard.Copy(content.String()); err != nil {
		app.stateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Failed to copy to clipboard: %v", err),
		})
		return app.refreshUI()
	}
	
	// Success message
	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: fmt.Sprintf("Copied %s to clipboard.", description),
	})
	
	return app.refreshUI()
}

func (app *App) parseYankArgument(arg string) (count int, direction string) {
	count = 0
	direction = ""
	
	// Parse patterns like "2k", "3j", "5", "k", "j", "-2", "-1"
	i := 0
	isRelative := false
	
	// Check for relative positioning (starts with -)
	if i < len(arg) && arg[i] == '-' {
		isRelative = true
		i++
	}
	
	// Extract number
	for i < len(arg) && arg[i] >= '0' && arg[i] <= '9' {
		count = count*10 + int(arg[i]-'0')
		i++
	}
	
	// Extract direction (for non-relative positioning)
	if i < len(arg) && !isRelative {
		direction = string(arg[i])
	}
	
	// For relative positioning, set direction to indicate relative mode
	if isRelative {
		direction = "-"
	}
	
	// Default count to 1 if not specified
	if count == 0 {
		count = 1
	}
	
	return count, direction
}

