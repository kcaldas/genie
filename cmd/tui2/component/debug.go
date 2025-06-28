package component

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type DebugComponent struct {
	*BaseComponent
	stateAccessor types.IStateAccessor
	isVisible     bool
	onTab         func(g *gocui.Gui, v *gocui.View) error // Tab handler callback
}

func NewDebugComponent(gui types.IGuiCommon, state types.IStateAccessor) *DebugComponent {
	ctx := &DebugComponent{
		BaseComponent:   NewBaseComponent("debug", "debug", gui),
		stateAccessor: state,
		isVisible:     false,
	}
	
	// Configure DebugComponent specific properties
	ctx.SetTitle(" Debug ")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:  true,
		Editable:   false,
		Wrap:       true,
		Autoscroll: true,
		Highlight:  true,
		Frame:      true,
	})
	
	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			v.Highlight = true
			v.SelBgColor = gocui.ColorYellow
			v.SelFgColor = gocui.ColorBlack
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

func (c *DebugComponent) clearDebugMessages(g *gocui.Gui, v *gocui.View) error {
	if c.stateAccessor != nil {
		messages := c.stateAccessor.GetDebugMessages()
		for i := range messages {
			messages[i] = ""
		}
		messages = messages[:0]
	}
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