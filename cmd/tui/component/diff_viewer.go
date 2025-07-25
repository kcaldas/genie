package component

import (
	"fmt"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type DiffViewerComponent struct {
	*BaseComponent
	*ScrollableBase
	content   string
	title     string
	isVisible bool
}

func NewDiffViewerComponent(gui types.Gui, title string, configManager *helpers.ConfigManager, eventBus *events.CommandEventBus) *DiffViewerComponent {
	ctx := &DiffViewerComponent{
		BaseComponent: NewBaseComponent("diff-viewer", "diff-viewer", gui, configManager),
		title:         title,
		isVisible:     false,
	}

	// Initialize ScrollableBase with a getter for this component's view
	ctx.ScrollableBase = NewScrollableBase(ctx.GetView)

	// Configure DiffViewerComponent specific properties
	ctx.SetTitle(fmt.Sprintf(" %s ", title))
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:   true,
		Editable:    false,
		Wrap:        false, // Don't wrap for diffs to preserve formatting
		Autoscroll:  false, // Allow manual scrolling for diff content
		Highlight:   true,
		Frame:       true,
		BorderStyle: types.BorderStyleSingle,
		FocusStyle:  types.FocusStyleBorder,
	})

	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			v.Highlight = true
			// Use theme colors for focus state
			theme := ctx.GetTheme()
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

	// Subscribe to command completion events that show diffs in diff viewer
	diffViewerUpdateHandler := func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.Render()
		})
	}

	// Add subscriptions for commands that generate diffs
	// (Can add more as needed when diff-generating commands are implemented)
	_ = diffViewerUpdateHandler // Will be used when diff commands are implemented

	return ctx
}

func (c *DiffViewerComponent) GetKeybindings() []*types.KeyBinding {
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
			Key:     gocui.KeyArrowLeft,
			Handler: c.scrollLeft,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowRight,
			Handler: c.scrollRight,
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

func (c *DiffViewerComponent) Render() error {
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
		// Process diff content with theme colors
		formattedContent := c.FormatDiff(c.content)
		fmt.Fprint(v, formattedContent)
	}

	return nil
}

func (c *DiffViewerComponent) IsVisible() bool {
	return c.isVisible
}

func (c *DiffViewerComponent) SetVisible(visible bool) {
	c.isVisible = visible
}

func (c *DiffViewerComponent) ToggleVisibility() {
	c.isVisible = !c.isVisible
}

// SetContent updates the diff content to display
func (c *DiffViewerComponent) SetContent(content string) {
	c.content = content
}

// GetContent returns the current diff content
func (c *DiffViewerComponent) GetContent() string {
	return c.content
}

// SetTitle updates the component title
func (c *DiffViewerComponent) SetTitle(title string) {
	c.title = title
	c.BaseComponent.SetTitle(fmt.Sprintf(" %s ", title))
}

// FormatDiff applies diff theme colors to diff content
func (c *DiffViewerComponent) FormatDiff(content string) string {
	config := c.GetConfig()
	
	// Get diff theme - use custom theme if set, otherwise use mapping
	var diffThemeName string
	if config.DiffTheme != "auto" {
		diffThemeName = config.DiffTheme
	} else {
		// Fall back to theme mapping if set to "auto"
		diffThemeName = presentation.GetDiffThemeForMainTheme(config.Theme)
	}
	
	diffTheme := presentation.GetDiffTheme(diffThemeName)
	formatter := presentation.NewDiffFormatter(diffTheme)
	return formatter.Format(content)
}

// Internal keybinding handlers
func (c *DiffViewerComponent) scrollUp(g *gocui.Gui, v *gocui.View) error {
	return c.ScrollUp()
}

func (c *DiffViewerComponent) scrollDown(g *gocui.Gui, v *gocui.View) error {
	return c.ScrollDown()
}

func (c *DiffViewerComponent) scrollLeft(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	if ox > 0 {
		return v.SetOrigin(ox-1, oy)
	}
	return nil
}

func (c *DiffViewerComponent) scrollRight(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	return v.SetOrigin(ox+1, oy)
}

func (c *DiffViewerComponent) pageUp(g *gocui.Gui, v *gocui.View) error {
	_, sy := v.Size()
	ox, oy := v.Origin()
	newY := oy - sy
	if newY < 0 {
		newY = 0
	}
	return v.SetOrigin(ox, newY)
}

func (c *DiffViewerComponent) pageDown(g *gocui.Gui, v *gocui.View) error {
	_, sy := v.Size()
	ox, oy := v.Origin()
	return v.SetOrigin(ox, oy+sy)
}

func (c *DiffViewerComponent) goToTop(g *gocui.Gui, v *gocui.View) error {
	ox, _ := v.Origin()
	return v.SetOrigin(ox, 0)
}

func (c *DiffViewerComponent) goToBottom(g *gocui.Gui, v *gocui.View) error {
	ox, _ := v.Origin()
	lines := len(v.BufferLines())
	_, sy := v.Size()
	maxY := lines - sy
	if maxY < 0 {
		maxY = 0
	}
	return v.SetOrigin(ox, maxY)
}
