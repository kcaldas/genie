package component

import (
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type InputComponent struct {
	*BaseComponent
	onSubmit      func(input types.UserInput) error
	historyIndex  int
	commandHistory []string
}

func NewInputComponent(gui types.IGuiCommon, onSubmit func(types.UserInput) error) *InputComponent {
	ctx := &InputComponent{
		BaseComponent:    NewBaseComponent("input", "input", gui),
		onSubmit:       onSubmit,
		historyIndex:   -1,
		commandHistory: []string{},
	}
	
	// Configure InputComponent specific properties
	ctx.SetTitle(" Input (/ for commands) ")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:  true,
		Editable:   true,
		Wrap:       true,
		Autoscroll: false,
		Highlight:  true,
		Frame:      true,
	})
	
	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			v.Highlight = true
			v.SelBgColor = gocui.ColorGreen
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
	}
}

func (c *InputComponent) handleSubmit(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(v.Buffer())
	if input == "" {
		return nil
	}
	
	c.addToHistory(input)
	c.historyIndex = -1
	
	v.Clear()
	v.SetCursor(0, 0)
	
	userInput := types.UserInput{
		Message:        input,
		IsSlashCommand: strings.HasPrefix(input, "/"),
	}
	
	if c.onSubmit != nil {
		return c.onSubmit(userInput)
	}
	
	return nil
}

func (c *InputComponent) navigateHistoryUp(g *gocui.Gui, v *gocui.View) error {
	if len(c.commandHistory) == 0 {
		return nil
	}
	
	if c.historyIndex < len(c.commandHistory)-1 {
		c.historyIndex++
		v.Clear()
		v.SetCursor(0, 0)
		v.Write([]byte(c.commandHistory[len(c.commandHistory)-1-c.historyIndex]))
	}
	
	return nil
}

func (c *InputComponent) navigateHistoryDown(g *gocui.Gui, v *gocui.View) error {
	if c.historyIndex > 0 {
		c.historyIndex--
		v.Clear()
		v.SetCursor(0, 0)
		v.Write([]byte(c.commandHistory[len(c.commandHistory)-1-c.historyIndex]))
	} else if c.historyIndex == 0 {
		c.historyIndex = -1
		v.Clear()
		v.SetCursor(0, 0)
	}
	
	return nil
}

func (c *InputComponent) clearInput(g *gocui.Gui, v *gocui.View) error {
	v.Clear()
	v.SetCursor(0, 0)
	c.historyIndex = -1
	return nil
}

func (c *InputComponent) addToHistory(input string) {
	if len(c.commandHistory) == 0 || c.commandHistory[len(c.commandHistory)-1] != input {
		c.commandHistory = append(c.commandHistory, input)
		if len(c.commandHistory) > 100 {
			c.commandHistory = c.commandHistory[1:]
		}
	}
}