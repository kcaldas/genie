package component

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type DebugComponent struct {
	*BaseComponent
	stateAccessor types.IStateAccessor
	isVisible     bool
	onTab         func(g *gocui.Gui, v *gocui.View) error // Tab handler callback
}

func NewDebugComponent(gui types.IGuiCommon, state types.IStateAccessor, eventBus *events.CommandEventBus) *DebugComponent {
	ctx := &DebugComponent{
		BaseComponent:   NewBaseComponent("debug", "debug", gui),
		stateAccessor: state,
		isVisible:     false,
	}
	
	// Configure DebugComponent specific properties
	ctx.SetTitle(" Debug ")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:   true,
		Editable:    false,
		Wrap:        true,
		Autoscroll:  true,
		Highlight:   true,
		Frame:       true,
		BorderStyle: types.BorderStyleSingle, // Debug panel uses single border
		FocusStyle:  types.FocusStyleBorder,  // Show focus via border color
	})
	
	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			v.Highlight = true
			// Use theme colors for focus state
			theme := ctx.gui.GetTheme()
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
	
	// Subscribe to command completion events that affect debug display
	eventBus.Subscribe("command.debug.executed", func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.Render()
		})
	})

	// Subscribe to theme changes
	eventBus.Subscribe("theme.changed", func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.Render()
		})
	})
	
	return ctx
}

func (c *DebugComponent) GetKeybindings() []*types.KeyBinding {
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
			Key:     'c',
			Handler: c.clearDebugMessages,
		},
		{
			View:    c.viewName,
			Key:     'y',
			Handler: c.copyDebugMessages,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyTab,
			Handler: c.handleTab,
		},
	}
}

func (c *DebugComponent) Render() error {
	if !c.isVisible {
		return nil
	}
	
	v := c.GetView()
	if v == nil {
		return nil
	}
	
	v.Clear()
	
	messages := c.stateAccessor.GetDebugMessages()
	for _, msg := range messages {
		fmt.Fprintln(v, msg)
	}
	
	// Auto-scroll to bottom if there are messages
	if len(messages) > 0 {
		c.scrollToBottom()
	}
	
	return nil
}

func (c *DebugComponent) IsVisible() bool {
	return c.isVisible
}

func (c *DebugComponent) SetVisible(visible bool) {
	c.isVisible = visible
}

func (c *DebugComponent) ToggleVisibility() {
	c.isVisible = !c.isVisible
}

func (c *DebugComponent) scrollUp(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	if oy > 0 {
		return v.SetOrigin(ox, oy-1)
	}
	return nil
}

func (c *DebugComponent) scrollDown(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	return v.SetOrigin(ox, oy+1)
}

func (c *DebugComponent) scrollToBottom() {
	v := c.GetView()
	if v == nil {
		return
	}
	
	// Get the number of lines in the buffer
	lines := strings.Split(v.Buffer(), "\n")
	lineCount := len(lines)
	
	// Get view height
	_, viewHeight := v.Size()
	
	// Calculate the origin to show the bottom of the content
	if lineCount > viewHeight {
		v.SetOrigin(0, lineCount-viewHeight)
	}
}

func (c *DebugComponent) clearDebugMessages(g *gocui.Gui, v *gocui.View) error {
	// Clear debug messages through the state accessor
	c.stateAccessor.ClearDebugMessages()
	return c.Render()
}

func (c *DebugComponent) copyDebugMessages(g *gocui.Gui, v *gocui.View) error {
	messages := c.stateAccessor.GetDebugMessages()
	_ = strings.Join(messages, "\n")
	// TODO: Implement clipboard functionality
	// For now, just log that we would copy
	return nil
}

func (c *DebugComponent) handleTab(g *gocui.Gui, v *gocui.View) error {
	if c.onTab != nil {
		return c.onTab(g, v)
	}
	return nil
}

func (c *DebugComponent) SetTabHandler(handler func(g *gocui.Gui, v *gocui.View) error) {
	c.onTab = handler
}