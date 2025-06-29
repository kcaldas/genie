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
		return nil
	})
	return nil
}




// Hidden debug commands (kept for development purposes)

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