package component

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type MessagesComponent struct {
	*BaseComponent
	stateAccessor    types.IStateAccessor
	messageFormatter *presentation.MessageFormatter
	onTab            func(g *gocui.Gui, v *gocui.View) error // Tab handler callback
}

func NewMessagesComponent(gui types.IGuiCommon, state types.IStateAccessor, eventBus *events.CommandEventBus) *MessagesComponent {
	mf, err := presentation.NewMessageFormatter(gui.GetConfig(), gui.GetTheme())
	if err != nil {
		panic("Unable to instantiate message formatter")
	}
	ctx := &MessagesComponent{
		BaseComponent:    NewBaseComponent("messages", "messages", gui),
		stateAccessor:    state,
		messageFormatter: mf,
	}

	// Configure MessagesComponent specific properties based on config
	config := gui.GetConfig()
	showBorder := config.ShowMessagesBorder

	if showBorder {
		ctx.SetTitle(" Chat ")
	} else {
		ctx.SetTitle("")
	}

	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:   true,
		Editable:    false, // Back to non-editable
		Wrap:        true,
		Autoscroll:  true,
		Highlight:   true,
		Frame:       showBorder,
		BorderStyle: types.BorderStyleSingle,
		FocusStyle:  types.FocusStyleBorder,
	})

	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			v.Highlight = true
			v.Editable = false // Keep non-editable
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

	eventBus.Subscribe("command.yank.executed", func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.Render()
		})
	})

	eventBus.Subscribe("theme.changed", func(e interface{}) {
		// Recreate message formatter with new theme
		if mf, err := presentation.NewMessageFormatter(ctx.gui.GetConfig(), ctx.gui.GetTheme()); err == nil {
			ctx.messageFormatter = mf
			ctx.gui.PostUIUpdate(func() {
				ctx.Render()
			})
		}
	})

	return ctx
}

func (c *MessagesComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{
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
		formatted := c.messageFormatter.FormatMessageWithWidth(msg, width)
		fmt.Fprint(v, formatted)
	}

	c.ScrollToBottom()

	return nil
}

func (c *MessagesComponent) ScrollToBottom() {
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

func (c *MessagesComponent) handleTab(g *gocui.Gui, v *gocui.View) error {
	// Tab handling will be managed by the app's central coordinator
	if c.onTab != nil {
		return c.onTab(g, v)
	}
	return nil
}

func (c *MessagesComponent) SetTabHandler(handler func(g *gocui.Gui, v *gocui.View) error) {
	c.onTab = handler
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

// RefreshBorderSettings updates the border visibility based on current config
func (c *MessagesComponent) RefreshBorderSettings() {
	config := c.gui.GetConfig()
	showBorder := config.ShowMessagesBorder

	// Update window properties
	props := c.windowProperties
	props.Frame = showBorder
	c.SetWindowProperties(props)

	// Update title based on border setting
	if showBorder {
		c.SetTitle(" Messages ")
	} else {
		c.SetTitle("")
	}

	// If we have a view, update its frame setting
	if view := c.GetView(); view != nil {
		view.Frame = showBorder
	}
}
