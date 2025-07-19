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
	component.SetTitle("Write Mode - Compose Message")
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

	// Subscribe to vim mode changes to refresh keybindings
	commandEventBus.Subscribe("vim.mode.changed", func(e interface{}) {
		component.RefreshKeybindings()
		component.RefreshEditor()
	})

	return component
}

func (c *WriteComponent) GetKeybindings() []*types.KeyBinding {
	config := c.GetConfig()
	keybindings := []*types.KeyBinding{
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlS,
			Handler: c.handleSubmit,
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

	// Only add ESC keybinding if NOT in vim mode
	if !config.VimMode {
		keybindings = append(keybindings, &types.KeyBinding{
			View:    c.viewName,
			Key:     gocui.KeyEsc,
			Handler: c.handleCancel,
		})
	}

	return keybindings
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
	config := c.GetConfig()

	// In vim mode, ESC behavior depends on current editor state
	if config.VimMode && v.Editor != nil {
		// If we're using vi editor, let it handle ESC (mode switching)
		if viEditor, ok := v.Editor.(*ViEditor); ok {
			// Let vi editor handle ESC first
			viEditor.Edit(v, gocui.KeyEsc, 0, gocui.ModNone)
			// Only close if we're in normal mode after ESC (not insert/command mode)
			if viEditor.GetMode() != NormalMode {
				return nil
			}
		}
	}

	// Default behavior: close the write component
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
	view.Subtitle = "Ctrl+S: Submit | Esc: Cancel | Ctrl+C/L: Clear"
	view.Editable = true
	view.Wrap = true
	view.Highlight = true
	view.Frame = true

	// Configure editor based on vim mode
	c.setupEditor(view)

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

// handleVimCommand handles vim commands like :q and :w
func (c *WriteComponent) handleVimCommand(command string) error {
	switch command {
	case "q":
		// Quit without saving
		if c.onClose != nil {
			return c.onClose()
		}
	case "w":
		// Write (save/submit)
		if view := c.GetView(); view != nil {
			return c.handleSubmit(c.gui.GetGui(), view)
		}
	case "wq":
		// Write and quit
		if view := c.GetView(); view != nil {
			if err := c.handleSubmit(c.gui.GetGui(), view); err != nil {
				return err
			}
		}
		if c.onClose != nil {
			return c.onClose()
		}
	}
	return nil
}

// RefreshKeybindings updates the keybindings when configuration changes
func (c *WriteComponent) RefreshKeybindings() {
	gui := c.gui.GetGui()
	if gui == nil {
		return
	}

	// Clear existing keybindings for this view
	gui.DeleteKeybindings(c.viewName)

	// Set up new keybindings based on current config
	for _, kb := range c.GetKeybindings() {
		gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler)
	}
}

// RefreshEditor updates the editor when vim mode changes
func (c *WriteComponent) RefreshEditor() {
	view := c.GetView()
	if view == nil {
		return
	}

	// Reuse the same editor setup logic
	c.setupEditor(view)
}

// setupEditor configures the appropriate editor based on vim mode
func (c *WriteComponent) setupEditor(view *gocui.View) {
	config := c.GetConfig()
	if config.VimMode {
		viEditor := NewViEditor().(*ViEditor)
		// Set up command handler for :q and :w
		viEditor.SetCommandHandler(c.handleVimCommand)
		// Set up mode change handler to update display
		viEditor.SetModeChangeHandler(func() {
			c.updateVimModeDisplay(view, viEditor)
		})
		view.Editor = viEditor
		// Update the mode display
		c.updateVimModeDisplay(view, viEditor)
	} else {
		view.Editor = NewCustomEditor()
		// Clear the title when not in vim mode
		view.Title = c.GetTitle()
	}
}

// updateVimModeDisplay updates the view's subtitle to show current vim mode
func (c *WriteComponent) updateVimModeDisplay(view *gocui.View, viEditor *ViEditor) {
	if view == nil || viEditor == nil {
		return
	}

	// Use GUI update to ensure thread safety
	gui := c.gui.GetGui()
	if gui != nil {
		gui.Update(func(g *gocui.Gui) error {
			c.doUpdateVimModeDisplay(view, viEditor)
			return nil
		})
	} else {
		c.doUpdateVimModeDisplay(view, viEditor)
	}
}

// doUpdateVimModeDisplay performs the actual vim mode display update
func (c *WriteComponent) doUpdateVimModeDisplay(view *gocui.View, viEditor *ViEditor) {
	if view == nil || viEditor == nil {
		return
	}

	var modeStr string
	switch viEditor.GetMode() {
	case NormalMode:
		modeStr = "NORMAL"
	case InsertMode:
		modeStr = "INSERT"
	case CommandMode:
		// Show command buffer for command mode
		cmdBuffer := viEditor.GetCommandBuffer()
		if cmdBuffer == "" {
			modeStr = ":"
		} else {
			modeStr = ":" + cmdBuffer
		}
	}

	// Keep subtitle for shortcuts
	view.Subtitle = "Ctrl+S: Submit | Ctrl+C/L: Clear"

	// Use title for vim mode indicator
	if modeStr != "" {
		view.Title = "[" + modeStr + "]"
	} else {
		view.Title = c.GetTitle()
	}
}
