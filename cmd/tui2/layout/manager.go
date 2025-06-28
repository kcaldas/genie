package layout

import (
	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

// Panel name constants - using semantic names
const (
	PanelStatus   = "status"   // top panel
	PanelLeft     = "left"     // left panel (unused currently)
	PanelMessages = "messages" // center panel
	PanelDebug    = "debug"    // right panel
	PanelInput    = "input"    // bottom panel
)

type LayoutManager struct {
	windowManager *WindowManager
	config        *LayoutConfig
	gui           *gocui.Gui
	components    map[string]types.Component // Component map like SimpleLayout
	lastWidth     int                        // Track terminal width for resize detection
	lastHeight    int                        // Track terminal height for resize detection
}

type LayoutConfig struct {
	MessagesWeight int  // Weight for messages panel
	InputHeight    int  // Fixed height for input panel
	DebugWeight    int  // Weight for debug panel when visible
	StatusHeight   int  // Fixed height for status panel
	ShowSidebar    bool // Legacy field - kept for compatibility
	CompactMode    bool // Compact mode toggle
	MinPanelWidth  int  // Minimum panel width
	MinPanelHeight int  // Minimum panel height
}

type LayoutArgs struct {
	Width  int
	Height int
	Config *LayoutConfig
}

func NewLayoutManager(gui *gocui.Gui, config *LayoutConfig) *LayoutManager {
	return &LayoutManager{
		windowManager: NewWindowManager(gui),
		config:        config,
		gui:           gui,
		components:    make(map[string]types.Component), // Initialize component map
	}
}

func (lm *LayoutManager) GetDefaultConfig() *LayoutConfig {
	return &LayoutConfig{
		MessagesWeight: 3,    // Messages takes 3/4 of available space
		InputHeight:    3,    // Input panel fixed at 3 lines
		DebugWeight:    1,    // Debug takes 1/4 when visible
		StatusHeight:   2,    // Status bar 2 lines
		ShowSidebar:    true, // Legacy compatibility
		CompactMode:    false,
		MinPanelWidth:  20,
		MinPanelHeight: 3,
	}
}

// GetDefaultFivePanelConfig returns a config showing all 5 panels - same as SimpleLayout
func GetDefaultFivePanelConfig() *LayoutConfig {
	return &LayoutConfig{
		MessagesWeight: 2, // Center panel weight
		InputHeight:    4, // Input panel height
		DebugWeight:    1, // Right panel weight
		StatusHeight:   4, // Top/Bottom panel height
		ShowSidebar:    true,
		CompactMode:    false,
		MinPanelWidth:  20,
		MinPanelHeight: 3,
	}
}

func (lm *LayoutManager) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	// Skip layout if terminal is too small
	if maxX <= 0 || maxY <= 0 {
		return nil
	}

	// Detect resize and trigger message re-render if needed
	sizeChanged := (lm.lastWidth != maxX || lm.lastHeight != maxY)
	if sizeChanged {
		lm.lastWidth = maxX
		lm.lastHeight = maxY
	}

	args := LayoutArgs{
		Width:  maxX,
		Height: maxY,
		Config: lm.config,
	}

	rootBox := lm.buildLayoutTree(args)
	windowDimensions := boxlayout.ArrangeWindows(rootBox, 0, 0, maxX, maxY)

	// Create windows if they don't exist and update dimensions
	for windowName, dims := range windowDimensions {
		if lm.windowManager.GetWindow(windowName) == nil {
			lm.windowManager.CreateWindow(windowName, dims)
		} else {
			lm.windowManager.UpdateWindowDimensions(windowName, dims)
		}
	}

	if err := lm.createViews(args); err != nil {
		return err
	}

	// Re-render messages only on size change to handle wrapping
	if sizeChanged {
		if messagesComponent, ok := lm.components[PanelMessages]; ok {
			if renderableComponent, ok := messagesComponent.(interface{ Render() error }); ok {
				renderableComponent.Render()
			}
		}
	}

	return nil
}

func (lm *LayoutManager) buildLayoutTree(args LayoutArgs) *boxlayout.Box {
	// Use exact same structure as SimpleLayout
	panels := []*boxlayout.Box{}

	centerColumns := []*boxlayout.Box{}

	if _, ok := lm.components[PanelLeft]; ok {
		// LEFT panel
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelLeft,
			Weight: 1,
		})
	}

	if _, ok := lm.components[PanelMessages]; ok {
		// MESSAGES panel (center)
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelMessages,
			Weight: 2,
		})
	}

	if _, ok := lm.components[PanelDebug]; ok {
		// DEBUG panel (right)
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelDebug,
			Weight: 1,
		})
	}

	// Middle horizontal row
	middlePanels := boxlayout.Box{
		Direction: boxlayout.COLUMN,
		Weight:    1,
		Children:  centerColumns,
	}

	panels = append(panels, &middlePanels)

	if _, ok := lm.components[PanelInput]; ok {
		// INPUT panel (bottom)
		panels = append(panels, &boxlayout.Box{
			Window: PanelInput,
			Size:   3, // Fixed size like SimpleLayout
		})
	}

	if _, ok := lm.components[PanelStatus]; ok {
		// STATUS panel (bottom) - single line
		panels = append(panels, &boxlayout.Box{
			Window: PanelStatus,
			Size:   3, // True single line with awesome-gocui support
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

func (lm *LayoutManager) createViews(args LayoutArgs) error {
	// Create views for panels that have components registered
	for panelName, component := range lm.components {
		if window := lm.windowManager.GetWindow(panelName); window != nil {
			// Use the component's view name, not the panel name
			viewName := component.GetViewName()
			if view, err := lm.windowManager.CreateOrUpdateView(panelName, viewName); err != nil {
				return err
			} else if view != nil {
				// Configure view with component properties
				lm.configureViewFromComponent(view, component)
			}
		}
	}

	return nil
}

// configureViewFromComponent applies component properties to the view - same as SimpleLayout
func (lm *LayoutManager) configureViewFromComponent(view *gocui.View, component types.Component) {
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

func (lm *LayoutManager) SetConfig(config *LayoutConfig) {
	lm.config = config
}

func (lm *LayoutManager) GetConfig() *LayoutConfig {
	return lm.config
}

func (lm *LayoutManager) SetWindowComponent(panelName string, component types.Component) {
	lm.components[panelName] = component
}

func (lm *LayoutManager) GetWindowComponent(panelName string) types.Component {
	return lm.components[panelName]
}

// GetAvailablePanels returns panel names that have components assigned - same as SimpleLayout
func (lm *LayoutManager) GetAvailablePanels() []string {
	var panels []string
	for panelName := range lm.components {
		panels = append(panels, panelName)
	}
	return panels
}

func (lm *LayoutManager) SetFocus(windowName string) {
	if window := lm.windowManager.GetWindow(windowName); window != nil {
		if len(window.Views) > 0 && window.Views[0] != nil {
			view := window.Views[0]
			lm.gui.SetCurrentView(view.Name())
			// Set highlight if the component supports it
			if component := lm.components[windowName]; component != nil {
				props := component.GetWindowProperties()
				if props.Highlight {
					view.Highlight = true
				}
			}
		}
	}
}

// GetLastSize returns the last known terminal size
func (lm *LayoutManager) GetLastSize() (int, int) {
	return lm.lastWidth, lm.lastHeight
}
