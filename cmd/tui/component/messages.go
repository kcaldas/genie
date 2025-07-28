package component

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type MessagesComponent struct {
	*BaseComponent
	*ScrollableBase
	stateAccessor    *state.ChatState
	messageFormatter *presentation.MessageFormatter
}

func NewMessagesComponent(gui types.Gui, state *state.ChatState, configManager *helpers.ConfigManager, eventBus *events.CommandEventBus) *MessagesComponent {
	mf, err := presentation.NewMessageFormatter(configManager.GetConfig(), configManager.GetTheme())
	if err != nil {
		panic("Unable to instantiate message formatter")
	}
	ctx := &MessagesComponent{
		BaseComponent:    NewBaseComponent("messages", "messages", gui, configManager),
		stateAccessor:    state,
		messageFormatter: mf,
	}

	// Initialize ScrollableBase with a getter for this component's view
	ctx.ScrollableBase = NewScrollableBase(ctx.GetView)

	// Configure MessagesComponent specific properties based on config
	config := configManager.GetConfig()
	showBorder := config.IsShowMessagesBorderEnabled()

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

	eventBus.Subscribe("command.yank.executed", func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.Render()
		})
	})

	eventBus.Subscribe("theme.changed", func(e interface{}) {
		// Recreate message formatter with new theme
		if mf, err := presentation.NewMessageFormatter(ctx.GetConfig(), ctx.GetTheme()); err == nil {
			ctx.messageFormatter = mf
			ctx.gui.PostUIUpdate(func() {
				ctx.Render()
			})
		}
	})

	eventBus.Subscribe("persona.changed", func(e interface{}) {
		if eventData, ok := e.(map[string]interface{}); ok {
			if personaName, ok := eventData["name"].(string); ok {
				ctx.gui.PostUIUpdate(func() {
					ctx.updateTitleForPersona(personaName)
				})
			}
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
	}
}

func (c *MessagesComponent) Render() error {
	// Call base render to apply theme colors
	if err := c.BaseComponent.Render(); err != nil {
		return err
	}
	
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
	config := c.GetConfig()
	showBorder := config.IsShowMessagesBorderEnabled()

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

// updateTitleForPersona updates the component title to show the persona name
func (c *MessagesComponent) updateTitleForPersona(personaName string) {
	config := c.GetConfig()
	showBorder := config.IsShowMessagesBorderEnabled()

	if showBorder {
		title := fmt.Sprintf(" %s ", personaName)
		c.SetTitle(title)
	} else {
		c.SetTitle("")
	}

	// If we have a view, update its frame setting and force a redraw
	if view := c.GetView(); view != nil {
		view.Frame = showBorder
		// Force render to update title display
		c.Render()
	}
}
