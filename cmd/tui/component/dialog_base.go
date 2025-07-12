package component

import (
	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// DialogComponent provides base functionality for dialog overlays
type DialogComponent struct {
	*BaseComponent
	internalLayout *boxlayout.Box
	dialogViews    map[string]*gocui.View
	bounds         DialogBounds
	onClose        func() error
	isVisible      bool
}

// DialogBounds represents the position and size of a dialog
type DialogBounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

// NewDialogComponent creates a new dialog base component
func NewDialogComponent(key, viewName string, guiCommon types.Gui, configManager *helpers.ConfigManager, onClose func() error) *DialogComponent {
	ctx := NewBaseComponent(key, viewName, guiCommon, configManager)

	// Configure as a dialog/overlay
	ctx.SetControlledBounds(false)
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:  true,
		Editable:   false,
		Wrap:       false,
		Autoscroll: false,
		Highlight:  false,
		Frame:      true,
	})

	return &DialogComponent{
		BaseComponent: ctx,
		dialogViews:   make(map[string]*gocui.View),
		onClose:       onClose,
		isVisible:     false,
	}
}

// SetInternalLayout sets the boxlayout structure for the dialog
func (d *DialogComponent) SetInternalLayout(layout *boxlayout.Box) {
	d.internalLayout = layout
}

// CalculateDialogBounds calculates centered dialog position and size
func (d *DialogComponent) CalculateDialogBounds(widthPercent, heightPercent int, minWidth, minHeight, maxWidth, maxHeight int) DialogBounds {
	maxX, maxY := d.BaseComponent.gui.GetGui().Size()

	// Calculate dialog size as percentage of screen
	width := maxX * widthPercent / 100
	height := maxY * heightPercent / 100

	// Apply min/max constraints
	if width < minWidth {
		width = minWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	if height < minHeight {
		height = minHeight
	}
	if height > maxHeight {
		height = maxHeight
	}

	// Center the dialog
	x := (maxX - width) / 2
	y := (maxY - height) / 2

	return DialogBounds{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	}
}

// LayoutDialog arranges internal views using boxlayout
func (d *DialogComponent) LayoutDialog() error {
	if d.internalLayout == nil {
		return nil
	}

	// Calculate dialog bounds
	bounds := d.bounds

	// Use boxlayout to arrange internal windows
	// The dialog view has a frame, which takes 1 char on each side
	// The title bar takes 1 line at the top (part of the frame)
	// So content area starts at +1,+1 and ends at -2,-2 from dialog bounds

	// Content area dimensions (relative to 0,0)
	contentWidth := bounds.Width - 2   // Subtract left and right borders
	contentHeight := bounds.Height - 2 // Subtract top and bottom borders

	// Let boxlayout calculate relative positions starting from 0,0
	windowDimensions := boxlayout.ArrangeWindows(
		d.internalLayout,
		0, 0,
		contentWidth, contentHeight,
	)

	// Create or update views for each window
	gui := d.BaseComponent.gui.GetGui()
	for windowName, dims := range windowDimensions {
		viewName := d.getInternalViewName(windowName)
		// Convert relative dimensions to absolute coordinates
		// Add dialog position + border offset to get absolute coordinates
		absX0 := bounds.X + 1 + dims.X0
		absY0 := bounds.Y + 1 + dims.Y0
		absX1 := bounds.X + 1 + dims.X1 - 1 // -1 for gocui convention
		absY1 := bounds.Y + 1 + dims.Y1 - 1 // -1 for gocui convention

		view, err := gui.SetView(viewName, absX0, absY0, absX1, absY1, 0)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}

		// Configure view
		if view != nil {
			view.Frame = false // Internal views don't need frames
			view.Wrap = false
			view.Autoscroll = false
			view.Highlight = false // Disable highlighting
			view.Editable = false  // Disable cursor
			// Force these settings immediately
			gui.Update(func(g *gocui.Gui) error {
				if v, err := g.View(viewName); err == nil && v != nil {
					v.Highlight = false
					v.Editable = false
					// Make cursor invisible by setting it to background color
					v.BgColor = gocui.ColorDefault
					v.FgColor = gocui.ColorDefault
				}
				return nil
			})
			d.dialogViews[windowName] = view
		}
	}

	return nil
}

// getInternalViewName generates unique view names for internal dialog views
func (d *DialogComponent) getInternalViewName(windowName string) string {
	return d.viewName + "-" + windowName
}

// Show displays the dialog
func (d *DialogComponent) Show(bounds DialogBounds) error {
	d.bounds = bounds
	d.isVisible = true

	// Create main dialog frame
	gui := d.BaseComponent.gui.GetGui()
	view, err := gui.SetView(d.viewName, bounds.X, bounds.Y, bounds.X+bounds.Width-1, bounds.Y+bounds.Height-1, 0)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	if view != nil {
		view.Frame = true
		view.Title = d.GetTitle()
		view.Highlight = false // Disable highlighting on main dialog
		view.Editable = false  // Disable editing on main dialog
		d.SetView(view)
	}

	// Layout internal components
	return d.LayoutDialog()
}

// Hide closes the dialog and cleans up views
func (d *DialogComponent) Hide() error {
	if !d.isVisible {
		return nil
	}

	gui := d.BaseComponent.gui.GetGui()

	// Delete internal views safely
	for _, view := range d.dialogViews {
		if view != nil {
			// Safely delete view - ignore errors if view doesn't exist
			gui.DeleteView(view.Name())
		}
	}
	d.dialogViews = make(map[string]*gocui.View)

	// Delete main dialog view safely
	if d.GetView() != nil {
		// Safely delete main view - ignore errors if view doesn't exist
		gui.DeleteView(d.viewName)
		d.SetView(nil)
	}

	d.isVisible = false
	return nil
}

// IsVisible returns whether the dialog is currently shown
func (d *DialogComponent) IsVisible() bool {
	return d.isVisible
}

// GetInternalView returns a view by window name
func (d *DialogComponent) GetInternalView(windowName string) *gocui.View {
	return d.dialogViews[windowName]
}

// Close calls the onClose callback if set
func (d *DialogComponent) Close() error {
	// Hide the dialog first
	if err := d.Hide(); err != nil {
		return err
	}

	// Call onClose callback if set (this typically calls app.closeCurrentDialog)
	if d.onClose != nil {
		return d.onClose()
	}

	return nil
}

// Common dialog keybinding for closing
func (d *DialogComponent) GetCloseKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{
		{
			View:    d.viewName,
			Key:     gocui.KeyEsc,
			Mod:     gocui.ModNone,
			Handler: func(g *gocui.Gui, v *gocui.View) error { return d.Close() },
		},
		{
			View:    d.viewName,
			Key:     'q',
			Mod:     gocui.ModNone,
			Handler: func(g *gocui.Gui, v *gocui.View) error { return d.Close() },
		},
	}
}
