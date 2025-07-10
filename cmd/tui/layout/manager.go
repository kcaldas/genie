package layout

import (
	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// Panel name constants - using semantic names
const (
	PanelStatus     = "status"      // top panel
	PanelLeft       = "left"        // left panel (unused currently)
	PanelMessages   = "messages"    // center panel
	PanelDebug      = "debug"       // right panel (debug component)
	PanelTextViewer = "text-viewer" // right panel (text viewer component)
	PanelDiffViewer = "diff-viewer" // right panel (diff viewer component)
	PanelInput      = "input"       // bottom panel
)

type LayoutManager struct {
	config     *LayoutConfig
	gui        *gocui.Gui
	panels     map[string]*Panel // Panel map replacing both components and windows
	lastWidth  int               // Track terminal width for resize detection
	lastHeight int               // Track terminal height for resize detection
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
		config: config,
		gui:    gui,
		panels: make(map[string]*Panel), // Initialize panel map
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

	rootBox := lm.buildLayoutTree()
	panelDimensions := boxlayout.ArrangeWindows(rootBox, 0, 0, maxX, maxY)

	// Update panel dimensions and create/update views
	for panelName, dims := range panelDimensions {
		if panel := lm.panels[panelName]; panel != nil {
			if err := panel.UpdateDimensions(dims); err != nil {
				return err
			}
			if err := panel.CreateOrUpdateView(); err != nil {
				return err
			}
		}
	}

	// Re-render messages only on size change to handle wrapping
	if sizeChanged {
		if messagesPanel := lm.panels[PanelMessages]; messagesPanel != nil {
			messagesPanel.Render()
		}
	}

	return nil
}

func (lm *LayoutManager) buildLayoutTree() *boxlayout.Box {
	// Use exact same structure as SimpleLayout
	panels := []*boxlayout.Box{}

	centerColumns := []*boxlayout.Box{}

	if panel := lm.panels[PanelLeft]; panel != nil && panel.IsVisible() {
		// LEFT panel
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelLeft,
			Weight: 1,
		})
	}

	if panel := lm.panels[PanelMessages]; panel != nil && panel.IsVisible() {
		// MESSAGES panel (center)
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelMessages,
			Weight: 2,
		})
	}

	// Check for any visible right panel component
	rightPanelAdded := false

	// Check debug panel
	if panel := lm.panels[PanelDebug]; panel != nil && panel.IsVisible() {
		centerColumns = append(centerColumns, &boxlayout.Box{
			Window: PanelDebug,
			Weight: 1,
		})
		rightPanelAdded = true
	}

	// Check text-viewer panel (only if debug not visible)
	if !rightPanelAdded {
		if panel := lm.panels[PanelTextViewer]; panel != nil && panel.IsVisible() {
			centerColumns = append(centerColumns, &boxlayout.Box{
				Window: PanelTextViewer,
				Weight: 1,
			})
			rightPanelAdded = true
		}
	}

	// Check diff-viewer panel (only if no other right panel visible)
	if !rightPanelAdded {
		if panel := lm.panels[PanelDiffViewer]; panel != nil && panel.IsVisible() {
			centerColumns = append(centerColumns, &boxlayout.Box{
				Window: PanelDiffViewer,
				Weight: 1,
			})
		}
	}

	// Middle horizontal row
	middlePanels := boxlayout.Box{
		Direction: boxlayout.COLUMN,
		Weight:    1,
		Children:  centerColumns,
	}

	panels = append(panels, &middlePanels)

	if panel := lm.panels[PanelInput]; panel != nil && panel.IsVisible() {
		// INPUT panel (bottom)
		panels = append(panels, &boxlayout.Box{
			Window: PanelInput,
			Size:   3, // Fixed size like SimpleLayout
		})
	}

	if panel := lm.panels[PanelStatus]; panel != nil && panel.IsVisible() {
		// STATUS panel (bottom) - single line with sub-sections
		statusColumns := []*boxlayout.Box{
			{
				Window: "status-left",
				Weight: 2,
			},
			{
				Window: "status-center",
				Weight: 1,
			},
			{
				Window: "status-right",
				Weight: 1,
			},
		}

		panels = append(panels, &boxlayout.Box{
			Window:    PanelStatus,
			Size:      1,
			Direction: boxlayout.COLUMN,
			Children:  statusColumns,
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

func (lm *LayoutManager) SetConfig(config *LayoutConfig) {
	lm.config = config
}

func (lm *LayoutManager) GetConfig() *LayoutConfig {
	return lm.config
}

func (lm *LayoutManager) SetComponent(panelName string, component types.Component) {
	panel := NewPanel(panelName, component, lm.gui)
	lm.panels[panelName] = panel
}

func (lm *LayoutManager) GetComponent(panelName string) types.Component {
	if panel := lm.panels[panelName]; panel != nil {
		return panel.Component
	}
	return nil
}

func (lm *LayoutManager) GetPanel(panelName string) *Panel {
	return lm.panels[panelName]
}

// GetAvailablePanels returns panel names that have components assigned
func (lm *LayoutManager) GetAvailablePanels() []string {
	var panelNames []string
	for panelName := range lm.panels {
		panelNames = append(panelNames, panelName)
	}
	return panelNames
}

func (lm *LayoutManager) SetFocus(panelName string) {
	if panel := lm.panels[panelName]; panel != nil && panel.View != nil {
		lm.gui.SetCurrentView(panel.View.Name())
		// Set highlight if the component supports it
		if panel.Component != nil {
			props := panel.Component.GetWindowProperties()
			if props.Highlight {
				panel.View.Highlight = true
			}
		}
	}
}

// GetLastSize returns the last known terminal size
func (lm *LayoutManager) GetLastSize() (int, int) {
	return lm.lastWidth, lm.lastHeight
}

// SwapComponent replaces a panel's component with a new one (for confirmations)
func (lm *LayoutManager) SwapComponent(panelName string, newComponent types.Component) error {
	if panel := lm.panels[panelName]; panel != nil {
		return panel.SwapComponent(newComponent)
	}
	// If panel doesn't exist, create it
	lm.SetComponent(panelName, newComponent)
	return nil
}

// AddSubPanel adds a sub-panel to a composite component (like status bar)
func (lm *LayoutManager) AddSubPanel(parentPanelName, subPanelName string, component types.Component) {
	if panel := lm.panels[parentPanelName]; panel != nil {
		panel.AddSubPanel(subPanelName, component)
	}
	// Also register the sub-panel as a standalone panel for boxlayout
	lm.SetComponent(subPanelName, component)
}

// GetNavigableViews returns view names in tab navigation order, excluding invisible panels and status
func (lm *LayoutManager) GetNavigableViews() []string {
	views := []string{}
	
	// Define navigation order (status excluded from tab navigation)
	panelOrder := []string{"input", "messages", "debug", "text-viewer", "diff-viewer", "left"}
	
	for _, panelName := range panelOrder {
		if panel := lm.panels[panelName]; panel != nil && panel.IsVisible() {
			if component := panel.Component; component != nil {
				viewName := component.GetViewName()
				views = append(views, viewName)
			}
		}
	}
	
	return views
}

// GetNextViewName returns the next view name in navigation order
func (lm *LayoutManager) GetNextViewName(currentViewName string) string {
	views := lm.GetNavigableViews()
	if len(views) == 0 {
		return ""
	}
	
	// If no current view, return first view
	if currentViewName == "" {
		return views[0]
	}
	
	// Find current view and return next
	for i, viewName := range views {
		if viewName == currentViewName {
			nextIndex := (i + 1) % len(views)
			return views[nextIndex]
		}
	}
	
	// If current view not found, return first view
	return views[0]
}
