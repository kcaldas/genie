package tui2

import (
	"fmt"

	"github.com/charmbracelet/glamour"
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

// refreshUI updates all components in the TUI
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
		// Always scroll to bottom after rendering messages
		// This ensures system messages from commands are visible
		return app.scrollToBottomMessages()
	})
	return nil
}

// refreshComponentThemes updates theme colors for all components and messageFormatter
func (app *App) refreshComponentThemes() error {
	// Wrap the theme refresh in a GUI update to ensure proper rendering
	app.gui.Update(func(g *gocui.Gui) error {
		// Update global GUI frame colors
		config := app.uiState.GetConfig()
		theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
		if theme != nil {
			g.FrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderDefault)
			g.SelFrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderFocused)
			
			// Update message formatter with new theme and glamour style
			var err error
			app.messageFormatter, err = presentation.NewMessageFormatter(config, theme)
			if err != nil {
				return err
			}
		}
		
		// Refresh border colors for all components
		app.messagesComponent.RefreshThemeColors()
		app.inputComponent.RefreshThemeColors()
		app.debugComponent.RefreshThemeColors()
		app.statusComponent.RefreshThemeColors()
		
		return nil
	})
	
	return nil
}




// Hidden debug commands (kept for development purposes)

func (app *App) cmdThemeDebug(args []string) error {
	config := app.uiState.GetConfig()
	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
	
	debugInfo := fmt.Sprintf(`=== THEME DEBUG INFO ===
Current theme: %s
Output mode: %s

ACTIVE COLORS (mode-aware):
Border default: %s
Border focused: %s
Primary: %s
Secondary: %s
Error: %s

Glamour style: %s
Markdown rendering: %t

GUI FrameColor: %v
GUI SelFrameColor: %v

MODE SUPPORT:
Has Normal mode colors: %t
Has 256-color mode colors: %t
Has TrueColor mode colors: %t
`, 
		config.Theme,
		config.OutputMode,
		theme.BorderDefault,
		theme.BorderFocused, 
		theme.Primary,
		theme.Secondary,
		theme.Error,
		presentation.GetGlamourStyleForTheme(config.Theme),
		config.MarkdownRendering,
		app.gui.FrameColor,
		app.gui.SelFrameColor,
		theme.Normal != nil,
		theme.Color256 != nil,
		theme.TrueColor != nil,
	)
	
	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: debugInfo,
	})
	
	// Force a theme refresh
	if err := app.refreshComponentThemes(); err != nil {
		app.stateAccessor.AddMessage(types.Message{
			Role:    "error", 
			Content: fmt.Sprintf("Theme refresh failed: %v", err),
		})
	} else {
		app.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: "Theme refresh completed successfully",
		})
	}
	
	return app.refreshUI()
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

func (app *App) cmdDiffDemo(args []string) error {
	// Sample diff content for testing
	sampleDiff := `diff --git a/example.go b/example.go
index 1234567..abcdefg 100644
--- a/example.go
+++ b/example.go
@@ -1,12 +1,15 @@
 package main
 
 import (
 	"fmt"
+	"log"
+	"os"
 )
 
 func main() {
-	fmt.Println("Hello, World!")
+	fmt.Println("Hello, Genie!")
+	log.Println("Application started")
 	
-	// TODO: Add more functionality
+	if len(os.Args) > 1 {
+		fmt.Printf("Args: %v\n", os.Args[1:])
+	}
 }
 
 func helper() {
-	// Old implementation
+	// New implementation with better error handling
+	if err := doSomething(); err != nil {
+		log.Fatal(err)
+	}
 }`
	
	title := "Sample Diff (example.go)"
	if len(args) > 0 {
		title = "Diff: " + args[0]
	}
	
	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: fmt.Sprintf("Showing diff in right panel: %s", title),
	})
	
	err := app.ShowDiffInViewer(sampleDiff, title)
	if err != nil {
		return err
	}
	
	return app.refreshUI()
}

