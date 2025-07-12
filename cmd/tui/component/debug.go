package component

import (
	"fmt"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type DebugComponent struct {
	*BaseComponent
	*ScrollableBase
	debugState *state.DebugState
	isVisible  bool
	eventBus   *events.CommandEventBus
	onTab      func(g *gocui.Gui, v *gocui.View) error // Tab handler callback
}

func NewDebugComponent(gui types.IGuiCommon, debugState *state.DebugState, configManager *helpers.ConfigManager, eventBus *events.CommandEventBus) *DebugComponent {
	ctx := &DebugComponent{
		BaseComponent: NewBaseComponent("debug", "debug", gui, configManager),
		debugState:    debugState,
		eventBus:      eventBus,
		isVisible:     false,
	}

	// Initialize ScrollableBase with a getter for this component's view
	ctx.ScrollableBase = NewScrollableBase(ctx.GetView)

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
	// Respect the visibility state - only render if component should be visible
	if !c.isVisible {
		return nil
	}

	v := c.GetView()
	if v == nil {
		// View doesn't exist yet, which can happen during layout creation
		return nil
	}

	v.Clear()

	messages := c.debugState.GetDebugMessages()
	for _, msg := range messages {
		fmt.Fprintln(v, msg)
	}

	// Auto-scroll to bottom if there are messages
	if len(messages) > 0 {
		c.ScrollToBottom()
	}

	return nil
}

// OnMessageAdded should be called when a new debug message is added to state
func (c *DebugComponent) OnMessageAdded() error {
	// If not visible, nothing to do - message is already in state
	if !c.isVisible {
		return nil
	}

	v := c.GetView()
	if v == nil {
		return nil
	}

	// Get the latest message (last one in the list with timestamp)
	messages := c.debugState.GetDebugMessages()
	if len(messages) == 0 {
		return nil
	}

	latestMessage := messages[len(messages)-1]

	// Append the new message with same formatting as Render()
	fmt.Fprintln(v, latestMessage)

	// Always auto-scroll to bottom to show the new message
	c.ScrollToBottom()

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
	return c.ScrollUp()
}

func (c *DebugComponent) scrollDown(g *gocui.Gui, v *gocui.View) error {
	return c.ScrollDown()
}

func (c *DebugComponent) clearDebugMessages(g *gocui.Gui, v *gocui.View) error {
	c.eventBus.Emit("debug.clear", "")
	return nil
}

func (c *DebugComponent) copyDebugMessages(g *gocui.Gui, v *gocui.View) error {
	c.eventBus.Emit("debug.copy", "")
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
