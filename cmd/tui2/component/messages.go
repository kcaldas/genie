package component

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type MessagesComponent struct {
	*BaseComponent
	stateAccessor types.IStateAccessor
	presentation  MessagePresenter
	onTab         func(g *gocui.Gui, v *gocui.View) error // Tab handler callback
}

type MessagePresenter interface {
	FormatMessage(msg types.Message) string
	FormatMessageWithWidth(msg types.Message, width int) string
	FormatLoadingIndicator() string
}

func NewMessagesComponent(gui types.IGuiCommon, state types.IStateAccessor, presenter MessagePresenter) *MessagesComponent {
	ctx := &MessagesComponent{
		BaseComponent: NewBaseComponent("messages", "messages", gui),
		stateAccessor: state,
		presentation:  presenter,
	}
	
	// Configure MessagesComponent specific properties
	ctx.SetTitle(" Messages ")
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
			v.SelBgColor = gocui.ColorCyan
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

func (c *MessagesComponent) GetKeybindings() []*types.KeyBinding {
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
			Handler: c.scrollTop,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEnd,
			Handler: c.scrollBottom,
		},
		{
			View:    c.viewName,
			Key:     'y',
			Handler: c.copySelectedMessage,
		},
		{
			View:    c.viewName,
			Key:     'Y',
			Handler: c.copyAllMessages,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyTab,
			Handler: c.handleTab,
		},
	}
}

func (c *MessagesComponent) Render() error {
	v := c.GetView()
	if v == nil {
		return nil
	}
	
	v.Clear()
	
	// Get current view width for dynamic formatting
	width, _ := v.Size()
	
	messages := c.stateAccessor.GetMessages()
	for _, msg := range messages {
		formatted := c.presentation.FormatMessageWithWidth(msg, width)
		fmt.Fprint(v, formatted)
	}
	
	if c.stateAccessor.IsLoading() {
		fmt.Fprint(v, c.presentation.FormatLoadingIndicator())
	}
	
	// Auto-scroll to bottom if autoscroll is enabled
	if v.Autoscroll {
		c.scrollToBottom()
	}
	
	return nil
}

func (c *MessagesComponent) scrollToBottom() {
	v := c.GetView()
	if v == nil {
		return
	}
	
	content := v.ViewBuffer()
	lines := strings.Count(content, "\n")
	_, height := v.Size()
	
	targetY := lines - height + 1
	if targetY < 0 {
		targetY = 0
	}
	
	v.SetOrigin(0, targetY)
}

func (c *MessagesComponent) scrollUp(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	if oy > 0 {
		return v.SetOrigin(ox, oy-1)
	}
	return nil
}

func (c *MessagesComponent) scrollDown(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	return v.SetOrigin(ox, oy+1)
}

func (c *MessagesComponent) pageUp(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	_, height := v.Size()
	newY := oy - height
	if newY < 0 {
		newY = 0
	}
	return v.SetOrigin(ox, newY)
}

func (c *MessagesComponent) pageDown(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()
	_, height := v.Size()
	return v.SetOrigin(ox, oy+height)
}

// Public methods for global access
func (c *MessagesComponent) PageUp(g *gocui.Gui, v *gocui.View) error {
	return c.pageUp(g, v)
}

func (c *MessagesComponent) PageDown(g *gocui.Gui, v *gocui.View) error {
	return c.pageDown(g, v)
}

func (c *MessagesComponent) handleTab(g *gocui.Gui, v *gocui.View) error {
	if c.onTab != nil {
		return c.onTab(g, v)
	}
	return nil
}

func (c *MessagesComponent) SetTabHandler(handler func(g *gocui.Gui, v *gocui.View) error) {
	c.onTab = handler
}

func (c *MessagesComponent) scrollTop(g *gocui.Gui, v *gocui.View) error {
	return v.SetOrigin(0, 0)
}

func (c *MessagesComponent) scrollBottom(g *gocui.Gui, v *gocui.View) error {
	content := v.ViewBuffer()
	lines := strings.Count(content, "\n")
	_, height := v.Size()
	
	targetY := lines - height + 1
	if targetY < 0 {
		targetY = 0
	}
	
	return v.SetOrigin(0, targetY)
}

func (c *MessagesComponent) copySelectedMessage(g *gocui.Gui, v *gocui.View) error {
	_, cy := v.Cursor()
	_, oy := v.Origin()
	lineNum := oy + cy
	
	messages := c.stateAccessor.GetMessages()
	if lineNum < len(messages) {
		_ = messages[lineNum].Content
		// TODO: Implement clipboard functionality
		// For now, just log that we would copy
	}
	return nil
}

func (c *MessagesComponent) copyAllMessages(g *gocui.Gui, v *gocui.View) error {
	messages := c.stateAccessor.GetMessages()
	var content strings.Builder
	
	for _, msg := range messages {
		content.WriteString(fmt.Sprintf("[%s]\n%s\n\n", msg.Role, msg.Content))
	}
	
	// TODO: Implement clipboard functionality
	// For now, just log that we would copy
	return nil
}