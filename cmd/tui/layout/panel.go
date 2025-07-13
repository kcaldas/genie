package layout

import (
	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// Panel represents a single layout area that contains a component and its view
type Panel struct {
	Name       string
	Component  types.Component
	Dimensions boxlayout.Dimensions
	View       *gocui.View
	Visible    bool
	gui        *gocui.Gui

	// For composite components like status bar
	SubPanels map[string]*Panel
}

// NewPanel creates a new panel with the given component
func NewPanel(name string, component types.Component, gui *gocui.Gui) *Panel {
	return &Panel{
		Name:      name,
		Component: component,
		Visible:   true,
		gui:       gui,
		SubPanels: make(map[string]*Panel),
	}
}

// UpdateDimensions updates the panel's dimensions and propagates to the view
func (p *Panel) UpdateDimensions(dims boxlayout.Dimensions) error {
	p.Dimensions = dims

	// Update main view if it exists
	if p.View != nil {
		if err := p.updateViewDimensions(); err != nil {
			return err
		}
	}

	// Update sub-panels if this is a composite component
	for _, subPanel := range p.SubPanels {
		if subPanel.View != nil {
			if err := subPanel.updateViewDimensions(); err != nil {
				return err
			}
		}
	}

	return nil
}

// updateViewDimensions updates the view's dimensions to match the panel
func (p *Panel) updateViewDimensions() error {
	if p.View == nil {
		return nil
	}

	_, err := p.gui.SetView(p.View.Name(), p.Dimensions.X0, p.Dimensions.Y0,
		p.Dimensions.X1-1, p.Dimensions.Y1, 0)
	return err
}

// CreateOrUpdateView creates or updates the gocui view for this panel
func (p *Panel) CreateOrUpdateView() error {
	if p.Component == nil {
		return nil
	}

	viewName := p.Component.GetViewName()
	view, err := p.gui.SetView(viewName, p.Dimensions.X0, p.Dimensions.Y0,
		p.Dimensions.X1-1, p.Dimensions.Y1, 0)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	// Check if this is a newly created view
	isNewView := (err == gocui.ErrUnknownView)
	
	p.View = view
	p.configureView()

	// Only do initialization for new views
	if isNewView {
		// Link component to view
		if baseComp, ok := p.Component.(interface{ SetView(*gocui.View) }); ok {
			baseComp.SetView(view)
		}

		// Set keybindings only once when view is created
		return p.setKeybindings()
	}
	
	return nil
}

func (p *Panel) setKeybindings() error {
	for _, kb := range p.Component.GetKeybindings() {
		if err := p.gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
			return err
		}
	}
	return nil
}

// configureView applies component properties to the view
func (p *Panel) configureView() {
	if p.View == nil || p.Component == nil {
		return
	}

	props := p.Component.GetWindowProperties()
	title := p.Component.GetTitle()

	// Apply frameOffset adjustment for frameless views
	if !props.Frame {
		frameOffset := 1
		x0, y0, x1, y1 := p.View.Dimensions()
		p.gui.SetView(p.View.Name(), x0-frameOffset, y0-frameOffset, x1+frameOffset, y1+frameOffset, 0)
	}

	// Apply component properties to view
	p.View.Title = title
	p.View.Editable = props.Editable
	p.View.Wrap = props.Wrap
	p.View.Highlight = props.Highlight
	p.View.Frame = props.Frame
}

// Render renders the panel's component if visible
func (p *Panel) Render() error {
	if !p.Visible || p.Component == nil {
		return nil
	}

	// Create view if it doesn't exist
	if p.View == nil {
		if err := p.CreateOrUpdateView(); err != nil {
			return err
		}
	}

	// Render the component
	if renderableComponent, ok := p.Component.(interface{ Render() error }); ok {
		return renderableComponent.Render()
	}

	return nil
}

// SetVisible sets the panel's visibility
func (p *Panel) SetVisible(visible bool) {
	p.Visible = visible

	// If hiding the panel, delete its view
	if !visible && p.View != nil {
		p.gui.DeleteView(p.View.Name())
		p.View = nil
	}

	// Update component visibility if it supports it
	if visibleComponent, hasVisibility := p.Component.(interface{ SetVisible(bool) }); hasVisibility {
		visibleComponent.SetVisible(visible)
	}
}

// IsVisible returns whether the panel is visible
func (p *Panel) IsVisible() bool {
	if !p.Visible {
		return false
	}

	// Check component visibility if it supports it
	if visibleComponent, hasVisibility := p.Component.(interface{ IsVisible() bool }); hasVisibility {
		return visibleComponent.IsVisible()
	}

	return p.Visible
}

// AddSubPanel adds a sub-panel for composite components
func (p *Panel) AddSubPanel(name string, component types.Component) {
	subPanel := NewPanel(name, component, p.gui)
	p.SubPanels[name] = subPanel
}

// GetSubPanel returns a sub-panel by name
func (p *Panel) GetSubPanel(name string) *Panel {
	return p.SubPanels[name]
}

// SwapComponent replaces the panel's component with a new one
func (p *Panel) SwapComponent(newComponent types.Component) error {
	p.Component = newComponent

	// Re-create the view with the new component
	if p.View != nil {
		p.gui.DeleteView(p.View.Name())
		p.View = nil
	}

	// Create the view
	if err := p.CreateOrUpdateView(); err != nil {
		return err
	}

	// Always register keybindings when swapping components
	// This ensures the new component's keybindings are registered
	return p.setKeybindings()
}

