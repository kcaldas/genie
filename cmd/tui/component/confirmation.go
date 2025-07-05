package component

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type ConfirmationComponent struct {
	*BaseComponent
	
	// Confirmation state
	ExecutionID      string // Public so it can be updated
	message          string
	onConfirmation   func(executionID string, confirmed bool) error
}

func NewConfirmationComponent(gui types.IGuiCommon, executionID, message string, onConfirmation func(string, bool) error) *ConfirmationComponent {
	ctx := &ConfirmationComponent{
		BaseComponent:   NewBaseComponent("input", "input", gui), // Use same view name as input
		ExecutionID:     executionID,
		message:         message,
		onConfirmation:  onConfirmation,
	}

	// Configure ConfirmationComponent specific properties
	ctx.SetTitle(" " + message + " ")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:  true,
		Editable:   false, // Not editable - only responds to specific keys
		Wrap:       true,
		Autoscroll: false,
		Highlight:  false,
		Frame:      true,
	})

	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			v.Editable = false // Ensure it's not editable
			v.Clear()
			v.SetCursor(0, 0)
		}
		// Apply secondary border color
		ctx.applySecondaryBorder()
		return nil
	})

	return ctx
}

func (c *ConfirmationComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{
		// Yes options
		{
			View:    c.viewName,
			Key:     '1',
			Mod:     gocui.ModNone,
			Handler: c.handleConfirmYes,
		},
		{
			View:    c.viewName,
			Key:     'y',
			Mod:     gocui.ModNone,
			Handler: c.handleConfirmYes,
		},
		{
			View:    c.viewName,
			Key:     'Y',
			Mod:     gocui.ModNone,
			Handler: c.handleConfirmYes,
		},
		// No options
		{
			View:    c.viewName,
			Key:     '2',
			Mod:     gocui.ModNone,
			Handler: c.handleConfirmNo,
		},
		{
			View:    c.viewName,
			Key:     'n',
			Mod:     gocui.ModNone,
			Handler: c.handleConfirmNo,
		},
		{
			View:    c.viewName,
			Key:     'N',
			Mod:     gocui.ModNone,
			Handler: c.handleConfirmNo,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEsc,
			Mod:     gocui.ModNone,
			Handler: c.handleConfirmNo,
		},
	}
}

// handleConfirmYes handles "1", "y", "Y" key press for confirmation
func (c *ConfirmationComponent) handleConfirmYes(g *gocui.Gui, v *gocui.View) error {
	if c.onConfirmation != nil {
		return c.onConfirmation(c.ExecutionID, true)
	}
	return nil
}

// handleConfirmNo handles "2", "n", "N" and Esc key press for rejection
func (c *ConfirmationComponent) handleConfirmNo(g *gocui.Gui, v *gocui.View) error {
	if c.onConfirmation != nil {
		return c.onConfirmation(c.ExecutionID, false)
	}
	return nil
}

// SetConfirmationHandler sets the callback for confirmation responses
func (c *ConfirmationComponent) SetConfirmationHandler(handler func(executionID string, confirmed bool) error) {
	c.onConfirmation = handler
}

// HandleFocus overrides BaseComponent to use secondary color
func (c *ConfirmationComponent) HandleFocus() error {
	// Apply secondary border color directly
	c.applySecondaryBorder()
	
	// Call the parent's onFocus handler
	if c.onFocus != nil {
		return c.onFocus()
	}
	return nil
}

// HandleFocusLost overrides BaseComponent to use secondary color
func (c *ConfirmationComponent) HandleFocusLost() error {
	// Apply secondary border color directly
	c.applySecondaryBorder()
	
	// Call the parent's onFocusLost handler
	if c.onFocusLost != nil {
		return c.onFocusLost()
	}
	return nil
}

// RefreshThemeColors updates the border color to use current secondary theme color
func (c *ConfirmationComponent) RefreshThemeColors() {
	// Apply secondary border color directly
	c.applySecondaryBorder()
}

// applySecondaryBorder directly applies secondary color to view
func (c *ConfirmationComponent) applySecondaryBorder() {
	view := c.GetView()
	if view == nil {
		return
	}
	
	// Test with a hard-coded regular red color to see if the mechanism works
	frameColor := presentation.ConvertAnsiToGocuiColor("\033[31m") // Regular red
	view.FrameColor = frameColor
}