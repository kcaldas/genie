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

// Navigation order for TAB cycling - excludes status panel from navigation
var defaultNavigationOrder = []string{
	PanelInput,
	PanelMessages,
	PanelDebug,
	PanelTextViewer,
	PanelDiffViewer,
	PanelLeft,
}

type LayoutManager struct {
	config     *LayoutConfig
	gui        *gocui.Gui
	panels     map[string]*Panel // Panel map replacing both components and windows
	lastWidth  int               // Track terminal width for resize detection
	lastHeight int               // Track terminal height for resize detection

	// Right panel state management
	rightPanelVisible bool
	rightPanelMode    string // "debug", "text-viewer", or "diff-viewer"
	rightPanelZoomed  bool   // Whether right panel is zoomed (takes most of the space)
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

func NewDefaultLayoutConfig(config *types.Config) *LayoutConfig {
	return &LayoutConfig{
		MessagesWeight: 3,                                    // Messages panel weight (main content)
		InputHeight:    4,                                    // Input panel height
		DebugWeight:    1,                                    // Debug panel weight when shown
		StatusHeight:   2,                                    // Status bar height
		ShowSidebar:    config.Layout.IsShowSidebarEnabled(), // Keep legacy field
		CompactMode:    config.Layout.CompactMode,            // Keep compact mode
		MinPanelWidth:  config.Layout.MinPanelWidth,          // Keep minimum constraints
		MinPanelHeight: config.Layout.MinPanelHeight,
	}
}

func NewLayoutManager(gui *gocui.Gui, config *LayoutConfig) *LayoutManager {
	return &LayoutManager{
		config: config,
		gui:    gui,
		panels: make(map[string]*Panel), // Initialize panel map

		// Initialize right panel state
		rightPanelVisible: false,
		rightPanelMode:    "debug", // Default to debug mode
		rightPanelZoomed:  false,   // Default to normal size
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
	panels := []*boxlayout.Box{}

	// Build center columns (left + messages + right panel)
	centerColumns := lm.buildCenterColumns()
	if len(centerColumns) > 0 {
		panels = append(panels, &boxlayout.Box{
			Direction: boxlayout.COLUMN,
			Weight:    1,
			Children:  centerColumns,
		})
	}

	// Add input panel if visible
	if lm.isPanelVisible(PanelInput) {
		panels = append(panels, lm.createPanelBox(PanelInput, 3, 0))
	}

	// Add status panel if visible
	if lm.isPanelVisible(PanelStatus) {
		panels = append(panels, lm.buildStatusPanel())
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

// buildCenterColumns builds the horizontal layout for center area
func (lm *LayoutManager) buildCenterColumns() []*boxlayout.Box {
	columns := []*boxlayout.Box{}

	// Left panel
	if lm.isPanelVisible(PanelLeft) {
		columns = append(columns, lm.createPanelBox(PanelLeft, 0, 1))
	}

	// Messages panel (main content) - adjust weight based on zoom state
	if lm.isPanelVisible(PanelMessages) {
		messagesWeight := 2 // Default weight
		if lm.rightPanelZoomed {
			messagesWeight = 1 // Reduced weight when right panel is zoomed
		}
		columns = append(columns, lm.createPanelBox(PanelMessages, 0, messagesWeight))
	}

	// Right panel (only one visible at a time) - adjust weight based on zoom state
	if rightPanel := lm.getVisibleRightPanel(); rightPanel != "" {
		rightPanelWeight := 1 // Default weight
		if lm.rightPanelZoomed {
			rightPanelWeight = 4 // Much larger weight when zoomed
		}
		columns = append(columns, lm.createPanelBox(rightPanel, 0, rightPanelWeight))
	}

	return columns
}

// createPanelBox creates a boxlayout.Box for a panel with given size/weight
func (lm *LayoutManager) createPanelBox(panelName string, size, weight int) *boxlayout.Box {
	box := &boxlayout.Box{Window: panelName}
	if size > 0 {
		box.Size = size
	} else {
		box.Weight = weight
	}
	return box
}

// buildStatusPanel creates the status panel with sub-sections
func (lm *LayoutManager) buildStatusPanel() *boxlayout.Box {
	statusColumns := []*boxlayout.Box{
		{Window: "status-left", Weight: 2},
		{Window: "status-center", Weight: 1},
		{Window: "status-right", Size: 36},
	}

	return &boxlayout.Box{
		Window:    PanelStatus,
		Size:      1,
		Direction: boxlayout.COLUMN,
		Children:  statusColumns,
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

// Helper methods for building layout tree

// isPanelVisible checks if a panel exists and is visible
func (lm *LayoutManager) isPanelVisible(panelName string) bool {
	panel := lm.panels[panelName]
	return panel != nil && panel.IsVisible()
}

// getVisibleRightPanel returns the name of the visible right panel (priority order)
func (lm *LayoutManager) getVisibleRightPanel() string {
	rightPanels := []string{PanelDebug, PanelTextViewer, PanelDiffViewer}
	for _, panelName := range rightPanels {
		if lm.isPanelVisible(panelName) {
			return panelName
		}
	}
	return ""
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

// getNavigableViews returns view names in tab navigation order, excluding invisible panels and status
func (lm *LayoutManager) getNavigableViews() []string {
	views := []string{}

	for _, panelName := range defaultNavigationOrder {
		if lm.isPanelVisible(panelName) {
			if viewName := lm.getViewName(panelName); viewName != "" {
				views = append(views, viewName)
			}
		}
	}

	return views
}

// getNextViewName returns the next view name in navigation order
func (lm *LayoutManager) getNextViewName(currentViewName string) string {
	views := lm.getNavigableViews()
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

// FocusNextPanel focuses the next panel in navigation order and returns the focused panel name
// configureViewForFocus applies special configuration when focusing a view
func (lm *LayoutManager) configureViewForFocus(view *gocui.View, panelName string) {
	if panelName == PanelInput {
		view.Editable = true
		view.SetCursor(0, 0)
		lm.gui.Cursor = true
	}
}

// FocusNextPanel focuses the next panel in navigation order and returns the focused panel name
func (lm *LayoutManager) FocusNextPanel() string {
	currentPanelName := lm.getCurrentPanelName()
	nextPanelName := lm.getNextViewName(currentPanelName)

	if nextPanelName == "" {
		return "" // No navigable panels
	}

	if err := lm.FocusPanel(nextPanelName); err != nil {
		return "" // Focus failed
	}

	return nextPanelName
}

// getViewName safely gets the view name for a panel
func (lm *LayoutManager) getViewName(panelName string) string {
	if panel := lm.panels[panelName]; panel != nil && panel.Component != nil {
		return panel.Component.GetViewName()
	}
	return ""
}

// getCurrentPanelName returns the name of the currently focused panel
func (lm *LayoutManager) getCurrentPanelName() string {
	if currentView := lm.gui.CurrentView(); currentView != nil {
		return currentView.Name()
	}
	return ""
}

// handleFocusTransition manages focus lost/gained between panels
func (lm *LayoutManager) handleFocusTransition(fromPanel, toPanel string) {
	// Handle focus lost
	if fromPanel != "" {
		if panel := lm.panels[fromPanel]; panel != nil && panel.Component != nil {
			panel.Component.HandleFocusLost()
		}
	}

	// Handle focus gained
	if toPanel != "" {
		if panel := lm.panels[toPanel]; panel != nil && panel.Component != nil {
			panel.Component.HandleFocus()
		}
	}
}

// FocusPanel focuses the specified panel by name, handling focus transitions
func (lm *LayoutManager) FocusPanel(panelName string) error {
	currentPanelName := lm.getCurrentPanelName()

	// Handle focus transitions
	lm.handleFocusTransition(currentPanelName, panelName)

	panel := lm.panels[panelName]
	if panel == nil || panel.Component == nil {
		// Fallback: try to set focus directly (for dialog views)
		if view, err := lm.gui.SetCurrentView(panelName); err == nil && view != nil {
			view.Highlight = true
		}
		return nil
	}

	// Set current view and configure if needed
	if view, err := lm.gui.SetCurrentView(panelName); err == nil && view != nil {
		lm.configureViewForFocus(view, panelName)
	}

	return nil
}

// Right panel management methods

// ShowRightPanel shows the specified right panel mode
func (lm *LayoutManager) ShowRightPanel(mode string) {
	lm.rightPanelVisible = true
	lm.rightPanelMode = mode

	// Hide all right panel components first
	rightPanels := []string{PanelDebug, PanelTextViewer, PanelDiffViewer}
	for _, panelName := range rightPanels {
		if panel := lm.panels[panelName]; panel != nil {
			panel.SetVisible(false)
		}
	}

	// Show only the target component
	var targetPanel string
	switch mode {
	case "debug":
		targetPanel = PanelDebug
	case "text-viewer":
		targetPanel = PanelTextViewer
	case "diff-viewer":
		targetPanel = PanelDiffViewer
	}

	if targetPanel != "" {
		if panel := lm.panels[targetPanel]; panel != nil {
			panel.SetVisible(true)
			// Trigger render to update content
			if mode == "diff-viewer" {
				panel.Render()
			}
		}
	}
}

// HideRightPanel hides all right panel components
func (lm *LayoutManager) HideRightPanel() {
	lm.rightPanelVisible = false

	// Hide all right panel components
	rightPanels := []string{PanelDebug, PanelTextViewer, PanelDiffViewer}
	for _, panelName := range rightPanels {
		if panel := lm.panels[panelName]; panel != nil {
			panel.SetVisible(false)
		}
	}
}

// ToggleRightPanel toggles the right panel visibility
func (lm *LayoutManager) ToggleRightPanel() {
	if lm.rightPanelVisible {
		lm.HideRightPanel()
	} else {
		lm.ShowRightPanel(lm.rightPanelMode)
	}
}

// SwitchRightPanelMode switches the right panel mode
func (lm *LayoutManager) SwitchRightPanelMode(mode string) {
	lm.rightPanelMode = mode
	if lm.rightPanelVisible {
		lm.ShowRightPanel(mode)
	}
}

// IsRightPanelVisible returns whether the right panel is visible
func (lm *LayoutManager) IsRightPanelVisible() bool {
	return lm.rightPanelVisible
}

// GetRightPanelMode returns the current right panel mode
func (lm *LayoutManager) GetRightPanelMode() string {
	return lm.rightPanelMode
}

// ResetKeybindings resets keybindings for all panels
// This ensures that component keybindings are properly registered after swaps
func (lm *LayoutManager) ResetKeybindings() error {
	for _, panel := range lm.panels {
		if panel != nil && panel.Component != nil && panel.View != nil {
			if err := panel.setKeybindings(); err != nil {
				return err
			}
		}
	}
	return nil
}

// DisableAllKeybindings removes all keybindings from all panels
func (lm *LayoutManager) DisableAllKeybindings() {
	for _, panel := range lm.panels {
		if panel != nil && panel.View != nil {
			lm.gui.DeleteKeybindings(panel.View.Name())
		}
	}
}

// EnableAllKeybindings restores keybindings for all panels
func (lm *LayoutManager) EnableAllKeybindings() error {
	return lm.ResetKeybindings()
}

// ZoomRightPanel zooms the right panel to take most of the layout space
func (lm *LayoutManager) ZoomRightPanel() {
	if lm.rightPanelVisible && !lm.rightPanelZoomed {
		lm.rightPanelZoomed = true
		// Force layout refresh
		lm.gui.Update(func(g *gocui.Gui) error {
			return nil
		})

		// Re-render components that need width updates
		lm.reRenderAfterZoom()
	}
}

// UnzoomRightPanel returns the right panel to normal size
func (lm *LayoutManager) UnzoomRightPanel() {
	if lm.rightPanelZoomed {
		lm.rightPanelZoomed = false
		// Force layout refresh
		lm.gui.Update(func(g *gocui.Gui) error {
			return nil
		})

		// Re-render components that need width updates
		lm.reRenderAfterZoom()
	}
}

// ToggleRightPanelZoom toggles the right panel zoom state
func (lm *LayoutManager) ToggleRightPanelZoom() {
	if lm.rightPanelZoomed {
		lm.UnzoomRightPanel()
	} else {
		lm.ZoomRightPanel()
	}
}

// reRenderAfterZoom re-renders components that need width updates after zoom changes
func (lm *LayoutManager) reRenderAfterZoom() {
	// Use GUI update to ensure rendering happens after layout changes
	lm.gui.Update(func(g *gocui.Gui) error {
		// Re-render help if text-viewer is visible to update for new width
		if textViewerPanel := lm.panels[PanelTextViewer]; textViewerPanel != nil && textViewerPanel.IsVisible() {
			textViewerPanel.Render()
		}

		// Re-render messages and scroll to bottom after zoom
		if messagesPanel := lm.panels[PanelMessages]; messagesPanel != nil {
			messagesPanel.Render()

			// Cast to scrollable interface and scroll to bottom
			if scrollable, ok := messagesPanel.Component.(interface{ ScrollToBottom() error }); ok {
				scrollable.ScrollToBottom()
			}
		}
		return nil
	})
}

// IsRightPanelZoomed returns whether the right panel is currently zoomed
func (lm *LayoutManager) IsRightPanelZoomed() bool {
	return lm.rightPanelZoomed
}
