package tui2

import (
	"github.com/awesome-gocui/gocui"
)

// Bounds represents the area available for a component
type Bounds struct {
	X, Y, Width, Height int
}

// Component represents a UI component that can be rendered
type Component interface {
	Render(g *gocui.Gui, bounds Bounds) error
}

// View represents a gocui view component
type View struct {
	name   string
	setup  func(v *gocui.View) error
	render func(v *gocui.View) error
}

// NewView creates a new View component
func NewView(name string) *View {
	return &View{name: name}
}

// WithSetup adds a setup function to configure the view
func (v *View) WithSetup(setup func(view *gocui.View) error) *View {
	v.setup = setup
	return v
}

// WithRender adds a render function to update the view content
func (v *View) WithRender(render func(view *gocui.View) error) *View {
	v.render = render
	return v
}

// Render implements Component interface
func (v *View) Render(g *gocui.Gui, bounds Bounds) error {
	view, err := g.SetView(v.name, bounds.X, bounds.Y, bounds.X+bounds.Width-1, bounds.Y+bounds.Height-1, 0)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		// Setup the view on first creation
		if v.setup != nil {
			if err := v.setup(view); err != nil {
				return err
			}
		}
	}
	
	// Always call render function if provided
	if v.render != nil {
		if err := v.render(view); err != nil {
			return err
		}
	}
	
	return nil
}

// HPanel represents a horizontal panel with left and right components
type HPanel struct {
	Left  Component
	Right Component
	Split float64 // 0.0 to 1.0, represents the percentage for the left component
}

// NewHPanel creates a new horizontal panel
func NewHPanel(left, right Component, split float64) *HPanel {
	return &HPanel{
		Left:  left,
		Right: right,
		Split: split,
	}
}

// Render implements Component interface
func (h *HPanel) Render(g *gocui.Gui, bounds Bounds) error {
	if bounds.Width < 2 {
		return nil // Too narrow to split
	}
	
	leftWidth := int(float64(bounds.Width) * h.Split)
	rightWidth := bounds.Width - leftWidth - 1 // -1 for border between panels
	
	if leftWidth > 0 {
		leftBounds := Bounds{
			X:      bounds.X,
			Y:      bounds.Y,
			Width:  leftWidth,
			Height: bounds.Height,
		}
		if err := h.Left.Render(g, leftBounds); err != nil {
			return err
		}
	}
	
	if rightWidth > 0 {
		rightBounds := Bounds{
			X:      bounds.X + leftWidth + 1,
			Y:      bounds.Y,
			Width:  rightWidth,
			Height: bounds.Height,
		}
		if err := h.Right.Render(g, rightBounds); err != nil {
			return err
		}
	}
	
	return nil
}

// VPanel represents a vertical panel with top and bottom components
type VPanel struct {
	Top    Component
	Bottom Component
	Split  float64 // 0.0 to 1.0, represents the percentage for the top component
}

// NewVPanel creates a new vertical panel
func NewVPanel(top, bottom Component, split float64) *VPanel {
	return &VPanel{
		Top:    top,
		Bottom: bottom,
		Split:  split,
	}
}

// Render implements Component interface
func (v *VPanel) Render(g *gocui.Gui, bounds Bounds) error {
	if bounds.Height < 2 {
		return nil // Too short to split
	}
	
	topHeight := int(float64(bounds.Height) * v.Split)
	bottomHeight := bounds.Height - topHeight - 1 // -1 for border between panels
	
	if topHeight > 0 {
		topBounds := Bounds{
			X:      bounds.X,
			Y:      bounds.Y,
			Width:  bounds.Width,
			Height: topHeight,
		}
		if err := v.Top.Render(g, topBounds); err != nil {
			return err
		}
	}
	
	if bottomHeight > 0 {
		bottomBounds := Bounds{
			X:      bounds.X,
			Y:      bounds.Y + topHeight + 1,
			Width:  bounds.Width,
			Height: bottomHeight,
		}
		if err := v.Bottom.Render(g, bottomBounds); err != nil {
			return err
		}
	}
	
	return nil
}