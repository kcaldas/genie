package component

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type ConfirmationComponent struct {
	*BaseComponent

	// Confirmation state
	ExecutionID    string // Public so it can be updated
	message        string
	onConfirmation func(executionID string, confirmed bool) error
}

func NewConfirmationComponent(gui types.Gui, configManager *helpers.ConfigManager, executionID, message string, onConfirmation func(string, bool) error) *ConfirmationComponent {
	ctx := &ConfirmationComponent{
		BaseComponent:  NewBaseComponent("input", "input", gui, configManager), // Use same view name as input
		ExecutionID:    executionID,
		message:        message,
		onConfirmation: onConfirmation,
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
		// Yes keys
		{
			View:    c.viewName,
			Key:     '1',
			Handler: c.handleConfirmation(true),
		},
		{
			View:    c.viewName,
			Key:     'y',
			Handler: c.handleConfirmation(true),
		},
		{
			View:    c.viewName,
			Key:     'Y',
			Handler: c.handleConfirmation(true),
		},
		// No keys
		{
			View:    c.viewName,
			Key:     '2',
			Handler: c.handleConfirmation(false),
		},
		{
			View:    c.viewName,
			Key:     'n',
			Handler: c.handleConfirmation(false),
		},
		{
			View:    c.viewName,
			Key:     'N',
			Handler: c.handleConfirmation(false),
		},
	}
}

func (c *ConfirmationComponent) handleConfirmation(confirmed bool) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		if c.onConfirmation != nil {
			return c.onConfirmation(c.ExecutionID, confirmed)
		}
		return nil
	}
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
