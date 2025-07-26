package component

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

const pageScrollAmount = 5 // Number of lines to scroll for PgUp/PgDown

type TextViewerComponent struct {
	*BaseComponent
	*ScrollableBase
	content     string
	contentType string // "text" or "markdown"
	title       string
	isVisible   bool
}

func NewTextViewerComponent(gui types.Gui, title string, configManager *helpers.ConfigManager, eventBus *events.CommandEventBus) *TextViewerComponent {
	ctx := &TextViewerComponent{
		BaseComponent: NewBaseComponent("text-viewer", "text-viewer", gui, configManager),
		title:         title,
		isVisible:     false,
	}

	// Initialize ScrollableBase with a getter for this component's view
	ctx.ScrollableBase = NewScrollableBase(ctx.GetView)

	// Configure TextViewerComponent specific properties
	ctx.SetTitle(fmt.Sprintf(" %s ", title))
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:   true,
		Editable:    false,
		Wrap:        true,
		Autoscroll:  false, // Allow manual scrolling for text content
		Highlight:   true,
		Frame:       true,
		BorderStyle: types.BorderStyleSingle,
		FocusStyle:  types.FocusStyleBorder,
	})

	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			v.Highlight = true
			// Use theme colors for focus state
			theme := ctx.configManager.GetTheme()
			bg, fg := presentation.GetThemeFocusColors(theme)
			v.SelBgColor = bg
			v.SelFgColor = fg
		}
		return nil
	})

	ctx.SetOnFocusLost(func() error {
		if v := ctx.GetView(); v != nil {
			v.Highlight = false
		}
		return nil
	})

	return ctx
}

func (c *TextViewerComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowUp,
			Handler: c.scrollUp,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowDown,
			Handler: c.scrollDown,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyPgup,
			Handler: c.pageUp,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyPgdn,
			Handler: c.pageDown,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyHome,
			Handler: c.goToTop,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEnd,
			Handler: c.goToBottom,
		},
	}
}

func (c *TextViewerComponent) Render() error {
	if !c.isVisible {
		return nil
	}

	v := c.GetView()
	if v == nil {
		return nil
	}
	
	// Call base render to apply theme colors (after view exists)
	if err := c.BaseComponent.Render(); err != nil {
		return err
	}

	v.Clear()

	if c.content != "" {
		displayContent := c.content

		// Process markdown content if needed
		if c.contentType == "markdown" {
			displayContent = c.ProcessMarkdown(c.content)
		}

		fmt.Fprint(v, displayContent)
	}

	return nil
}

func (c *TextViewerComponent) IsVisible() bool {
	return c.isVisible
}

func (c *TextViewerComponent) SetVisible(visible bool) {
	c.isVisible = visible
}

func (c *TextViewerComponent) ToggleVisibility() {
	c.isVisible = !c.isVisible
}

// SetContent updates the text content to display
func (c *TextViewerComponent) SetContent(content string) {
	c.content = content
}

// SetContentWithType updates the text content and content type
func (c *TextViewerComponent) SetContentWithType(content, contentType string) {
	c.content = content
	c.contentType = contentType
}

// GetContent returns the current text content
func (c *TextViewerComponent) GetContent() string {
	return c.content
}

// GetContentType returns the content type
func (c *TextViewerComponent) GetContentType() string {
	return c.contentType
}

// SetTitle updates the component title
func (c *TextViewerComponent) SetTitle(title string) {
	c.title = title
	c.BaseComponent.SetTitle(fmt.Sprintf(" %s ", title))
}

// Internal keybinding handlers
func (c *TextViewerComponent) scrollUp(g *gocui.Gui, v *gocui.View) error {
	return c.ScrollUp()
}

func (c *TextViewerComponent) scrollDown(g *gocui.Gui, v *gocui.View) error {
	return c.ScrollDown()
}

func (c *TextViewerComponent) pageUp(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	newY := oy - pageScrollAmount
	if newY < 0 {
		newY = 0
	}
	return v.SetOrigin(ox, newY)
}

func (c *TextViewerComponent) pageDown(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	return v.SetOrigin(ox, oy+pageScrollAmount)
}

func (c *TextViewerComponent) goToTop(g *gocui.Gui, v *gocui.View) error {
	ox, _ := v.Origin()
	return v.SetOrigin(ox, 0)
}

func (c *TextViewerComponent) goToBottom(g *gocui.Gui, v *gocui.View) error {
	ox, _ := v.Origin()
	lines := len(v.BufferLines())
	_, sy := v.Size()
	maxY := lines - sy
	if maxY < 0 {
		maxY = 0
	}
	return v.SetOrigin(ox, maxY)
}

// ProcessMarkdown renders markdown using glamour like the message formatter
func (c *TextViewerComponent) ProcessMarkdown(content string) string {
	// Get view dimensions for rendering width
	v := c.GetView()
	width := 80 // Default width
	if v != nil {
		viewWidth, _ := v.Size()
		if viewWidth > 4 {
			width = viewWidth - 2 // Leave some margin
		}
	}

	// Get theme from gui common
	theme := c.GetTheme()
	config := c.GetConfig()

	// Smart theme selection based on content size to prevent ANSI bloat
	glamourTheme := config.GlamourTheme
	if len(content) > 50000 {  // Only use ASCII for extremely large content
		glamourTheme = "ascii"
	}
	
	renderer, err := presentation.CreateMarkdownRendererWithWidth(theme, config.Theme, glamourTheme, width)
	if err != nil {
		// Fallback to raw content if rendering fails
		return content
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		// Fallback to raw content if rendering fails
		return content
	}

	return strings.TrimSpace(rendered)
}
