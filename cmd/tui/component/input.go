package component

import (
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type InputComponent struct {
	*BaseComponent
	commandEventBus *events.CommandEventBus
	history         history.ChatHistory
	onTab           types.TabHandler // Tab handler callback
}

func NewInputComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, historyPath string, tabHandler types.TabHandler) *InputComponent {
	ctx := &InputComponent{
		BaseComponent:   NewBaseComponent("input", "input", gui, configManager),
		commandEventBus: commandEventBus,
		history:         history.NewChatHistory(historyPath, true), // Enable saving
		onTab:           tabHandler,
	}

	// Load history on startup
	if err := ctx.LoadHistory(); err != nil {
		// Don't fail startup if history loading fails, just log it
		// Since we're discarding logs in TUI mode, this won't show up
	}

	// Configure InputComponent specific properties
	ctx.SetTitle("")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:  true,
		Editable:   true,
		Wrap:       true,
		Autoscroll: false,
		Highlight:  false,
		Frame:      true,
	})

	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			//v.SelFgColor = gocui.ColorBlack
		}
		return nil
	})

	ctx.SetOnFocusLost(func() error {
		if v := ctx.GetView(); v != nil {
			//v.Highlight = false
		}
		return nil
	})

	// Subscribe to theme changes
	commandEventBus.Subscribe("theme.changed", func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.RefreshThemeColors()
		})
	})

	return ctx
}

func (c *InputComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{
		{
			View:    c.viewName,
			Key:     gocui.KeyEnter,
			Handler: c.handleSubmit,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowUp,
			Handler: c.navigateHistoryUp,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowDown,
			Handler: c.navigateHistoryDown,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlC,
			Handler: c.clearInput,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlL,
			Handler: c.clearInput,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyTab,
			Handler: c.handleTab,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEsc,
			Handler: c.handleEsc,
		},
	}
}

func (c *InputComponent) handleSubmit(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(v.Buffer())
	if input == "" {
		return nil
	}

	c.history.AddCommand(input)
	c.history.ResetNavigation()

	v.Clear()
	v.SetCursor(0, 0)

	// Determine if input is a command and emit appropriate event
	if strings.HasPrefix(input, ":") {
		// Emit command event
		c.commandEventBus.Emit("user.input.command", input)
	} else {
		// Emit text/chat message event
		c.commandEventBus.Emit("user.input.text", input)
	}

	return nil
}

func (c *InputComponent) handleEsc(g *gocui.Gui, v *gocui.View) error {
	c.commandEventBus.Emit("user.input.cancel", "")
	return nil
}

func (c *InputComponent) navigateHistoryUp(g *gocui.Gui, v *gocui.View) error {
	command := c.history.NavigatePrev()
	if command != "" {
		v.Clear()
		v.SetCursor(0, 0)
		v.Write([]byte(command))
		// Move cursor to end of text
		v.SetCursor(len(command), 0)
	}
	return nil
}

func (c *InputComponent) navigateHistoryDown(g *gocui.Gui, v *gocui.View) error {
	command := c.history.NavigateNext()
	v.Clear()
	v.SetCursor(0, 0)
	if command != "" {
		v.Write([]byte(command))
		// Move cursor to end of text
		v.SetCursor(len(command), 0)
	}
	return nil
}

func (c *InputComponent) clearInput(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(v.Buffer())

	// If input is empty, exit the application (Ctrl+C behavior)
	if input == "" {
		return gocui.ErrQuit
	}

	// Otherwise, clear the input (original behavior)
	v.Clear()
	v.SetCursor(0, 0)
	c.history.ResetNavigation()
	return nil
}

func (c *InputComponent) handleTab(g *gocui.Gui, v *gocui.View) error {
	if c.onTab != nil {
		return c.onTab(v)
	}
	return nil
}

func (c *InputComponent) SetTabHandler(handler types.TabHandler) {
	c.onTab = handler
}

func (c *InputComponent) LoadHistory() error {
	return c.history.Load()
}
