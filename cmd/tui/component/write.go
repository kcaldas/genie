package component

import (
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// WriteComponent provides a full-screen text area for composing longer messages
type WriteComponent struct {
	*BaseComponent
	commandEventBus *events.CommandEventBus
	history         history.ChatHistory
	onClose         func() error
}

func NewWriteComponent(
	gui types.Gui,
	configManager *helpers.ConfigManager,
	commandEventBus *events.CommandEventBus,
	onClose func() error,
) *WriteComponent {
	component := &WriteComponent{
		BaseComponent:   NewBaseComponent("write", "write", gui, configManager),
		commandEventBus: commandEventBus,
		history:         history.NewChatHistory(".genie/history", true),
		onClose:         onClose,
	}

	// Configure as a full-screen overlay like dialogs
	component.SetControlledBounds(false)
	component.SetTitle("Write Text Input (Ctrl+S to submit, Esc to cancel)")
	component.SetWindowProperties(types.WindowProperties{
		Focusable:  true,
		Editable:   true,
		Wrap:       true,
		Autoscroll: false,
		Highlight:  true,
		Frame:      true,
	})

	// Load history
	component.LoadHistory()

	return component
}

func (c *WriteComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlS,
			Handler: c.handleSubmit,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEsc,
			Handler: c.handleCancel,
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

func (c *WriteComponent) handleSubmit(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(v.Buffer())
	if input == "" {
		return c.handleCancel(g, v)
	}

	// Add to history
	c.history.AddCommand(input)

	// Emit text input event only
	c.commandEventBus.Emit("user.input.text", input)

	// Close the text area
	return c.handleCancel(g, v)
}

func (c *WriteComponent) handleCancel(g *gocui.Gui, v *gocui.View) error {
	if c.onClose != nil {
		return c.onClose()
	}
	return nil
}

func (c *WriteComponent) clearInput(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(v.Buffer())

	// If input is empty, cancel the text area
	if input == "" {
		return c.handleCancel(g, v)
	}

	// Otherwise, clear the input
	v.Clear()
	v.SetCursor(0, 0)
	return nil
}

func (c *WriteComponent) LoadHistory() error {
	return c.history.Load()
}

// SetInitialContent sets the initial content for the text area
// CreateView creates a full-screen overlay view for the text area
func (c *WriteComponent) CreateView() (*gocui.View, error) {
	gui := c.gui.GetGui()
	maxX, maxY := gui.Size()

	view, err := gui.SetView(c.viewName, 0, 0, maxX-2, maxY-2, 0)
	if err != nil && err != gocui.ErrUnknownView {
		return nil, err
	}

	// Configure the view
	view.Title = c.GetTitle()
	view.Editable = true
	view.Wrap = true
	view.Highlight = true
	view.Frame = true

	return view, nil
}

func (c *WriteComponent) SetInitialContent(content string) {
	if view := c.GetView(); view != nil {
		view.Clear()
		view.Write([]byte(content))
		// Position cursor at end
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			lastLine := lines[len(lines)-1]
			view.SetCursor(len(lastLine), len(lines)-1)
		}
	}
}

// Show displays the write input overlay
func (c *WriteComponent) Show() error {
	gui := c.gui.GetGui()

	// Note: Controller handles disabling all keybindings to ensure this component gets exclusive input

	// Create the view
	_, err := c.CreateView()
	if err != nil {
		return err
	}

	// Set up keybindings
	for _, kb := range c.GetKeybindings() {
		if err := gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
			return err
		}
	}

	// Enable cursor and focus the write input using gui.Update like LLM context viewer
	gui.Update(func(g *gocui.Gui) error {
		g.Cursor = true
		if v, err := g.View(c.viewName); err == nil {
			_, err := g.SetCurrentView(v.Name())
			if err == nil {
				v.Highlight = true
				// Explicitly set cursor position in the write input
				v.SetCursor(0, 0)
			}
			return err
		}
		return nil
	})

	return err
}

