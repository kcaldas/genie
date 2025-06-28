package tui2

import (
	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

// Panel name constants
const (
	PanelTop    = "top"
	PanelLeft   = "left"
	PanelCenter = "center"
	PanelRight  = "right"
	PanelBottom = "bottom"
)

// SimpleLayout - Just manages 5 views with basic operations
type SimpleLayout struct {
	gui        *gocui.Gui
	config     SimpleLayoutConfig
	components map[string]types.Component
}

// SimpleLayoutConfig defines which panels to show and their sizes
type SimpleLayoutConfig struct {
	TopHeight    int // Fixed height, 0 = hidden
	LeftWeight   int // % of window width (0.0-1.0), 0 = hidden
	CenterWeight int // Weight of center panel (0.0-1.0), 0 = hidden
	RightWeight  int // Weight of right panel (0.0-1.0), 0 = hidden
	BottomHeight int // Fixed height, 0 = hidden
}

func NewSimpleLayout(gui *gocui.Gui, config SimpleLayoutConfig) *SimpleLayout {
	return &SimpleLayout{
		gui:        gui,
		config:     config,
		components: make(map[string]types.Component),
	}
}

// Layout - Main layout function called by gocui
func (sl *SimpleLayout) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	// Skip layout if terminal is too small
	if maxX <= 0 || maxY <= 0 {
		return nil
	}

	// Use boxlayout with conditional toggles
	rootBox := sl.buildLayoutTree(maxX, maxY)
	windowDimensions := boxlayout.ArrangeWindows(rootBox, 0, 0, maxX, maxY)

	// Create/update views for each panel
	for panelName, dims := range windowDimensions {
		if err := sl.createOrUpdateView(panelName, dims); err != nil {
			return err
		}
	}

	return nil
}

// buildLayoutTree creates simple boxlayout structure
func (sl *SimpleLayout) buildLayoutTree(maxX, maxY int) *boxlayout.Box {
	// Simple static structure - no conditionals yet
	panels := []*boxlayout.Box{}

	if _, ok := sl.components[PanelTop]; ok {
		// TOP panel
		panels = append(panels, &boxlayout.Box{
			Window: PanelTop,
			Size:   sl.config.TopHeight,
		})
	}

	centerColumns := []*boxlayout.Box{}

	if _, ok := sl.components[PanelLeft]; ok {
		// LEFT panel
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelLeft,
			Weight: sl.config.LeftWeight,
		})
	}

	if _, ok := sl.components[PanelCenter]; ok {
		// CENTER panel
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelCenter,
			Weight: sl.config.CenterWeight,
		})
	}

	if _, ok := sl.components[PanelRight]; ok {
		// RIGHT panel
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelRight,
			Weight: sl.config.RightWeight,
		})
	}

	// Middle horizontal row
	middlePanels := boxlayout.Box{
		Direction: boxlayout.COLUMN,
		Weight:    1,
		Children:  centerColumns,
	}

	panels = append(panels, &middlePanels)

	if _, ok := sl.components[PanelBottom]; ok {
		// BOTTOM panel
		panels = append(panels, &boxlayout.Box{
			Window: PanelBottom,
			Size:   sl.config.BottomHeight,
		})
	}

	return &boxlayout.Box{
		Direction: boxlayout.COLUMN,
		Children: []*boxlayout.Box{
			{
				Direction: boxlayout.ROW,
				Weight:    1,
				Children:  panels,
			},
		},
	}
}

// createOrUpdateView creates or updates a basic view
func (sl *SimpleLayout) createOrUpdateView(panelName string, dims boxlayout.Dimensions) error {
	// Validate dimensions to prevent zero-size views
	width := dims.X1 - dims.X0
	height := dims.Y1 - dims.Y0
	if width <= 0 || height <= 0 {
		return nil
	}

	// Add minimum size constraint
	if width < 3 || height < 1 {
		return nil
	}

	// Create or get existing view
	v, err := sl.gui.SetView(panelName, dims.X0, dims.Y0, dims.X1-1, dims.Y1-1, 0)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	// Configure view with component if available, otherwise basic config
	if v != nil {
		if component, exists := sl.components[panelName]; exists {
			sl.configureViewFromComponent(v, component)
		} else {
			// Basic view configuration
			v.Frame = true
			v.Title = " " + panelName + " "
		}
	}

	return nil
}

// SetFocus sets focus to a specific panel
func (sl *SimpleLayout) SetFocus(panelName string) error {
	// Check if view exists before setting focus
	if _, err := sl.gui.View(panelName); err != nil {
		return err
	}
	_, err := sl.gui.SetCurrentView(panelName)
	return err
}

// SetView sets a component for a specific panel
func (sl *SimpleLayout) SetView(panelName string, component types.Component) {
	sl.components[panelName] = component

	// If view already exists, configure it with the component
	if view, err := sl.gui.View(panelName); err == nil {
		sl.configureViewFromComponent(view, component)
	}
}

// configureViewFromComponent applies component properties to the view
func (sl *SimpleLayout) configureViewFromComponent(view *gocui.View, component types.Component) {
	props := component.GetWindowProperties()
	title := component.GetTitle()

	// Apply component properties to view
	view.Title = title
	view.Editable = props.Editable
	view.Wrap = props.Wrap
	view.Autoscroll = props.Autoscroll
	view.Highlight = props.Highlight
	view.Frame = props.Frame

	// Link component to view (so component can render to it)
	if baseComp, ok := component.(interface{ SetView(*gocui.View) }); ok {
		baseComp.SetView(view)
	}
}

// GetAvailablePanels returns the names of all registered panels
func (sl *SimpleLayout) GetAvailablePanels() []string {
	panels := make([]string, 0, len(sl.components))
	for name := range sl.components {
		panels = append(panels, name)
	}
	return panels
}

// GetDefaultFivePanelConfig returns a config showing all 5 panels
func GetDefaultFivePanelConfig() SimpleLayoutConfig {
	return SimpleLayoutConfig{
		TopHeight:    4, // Top panel 4 lines
		LeftWeight:   1, // Left panel 1/5 of width
		CenterWeight: 2, // Center panel 2/5 of width
		RightWeight:  1, // Right panel 1/5 of width
		BottomHeight: 4, // Bottom panel 4 lines
	}
}
