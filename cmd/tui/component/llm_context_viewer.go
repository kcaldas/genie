package component

import (
	"fmt"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// LLMContextViewerComponent provides a full-screen modal for viewing LLM context data.
// It displays context parts organized by provider with dual-panel navigation similar
// to the previous TUI implementation.
type LLMContextViewerComponent struct {
	*BaseComponent
	dataProvider       types.LLMContextDataProvider
	selectedContextKey int
	contextKeys        []string               // Sorted list of context keys for navigation
	contentViewport    *ContextViewport       // For content scrolling
	internalViews      map[string]*gocui.View // Store our own view references
	internalLayout     *boxlayout.Box         // Layout definition
	onClose            func() error           // Close callback
	isVisible          bool                   // Visibility state
}

// ContextViewport handles scrolling within the content panel
type ContextViewport struct {
	offsetY    int
	maxY       int
	viewHeight int
}

func NewLLMContextViewerComponent(guiCommon types.IGuiCommon, dataProvider types.LLMContextDataProvider, onClose func() error) *LLMContextViewerComponent {
	component := &LLMContextViewerComponent{
		BaseComponent:      NewBaseComponent("llm-context-viewer", "llm-context-viewer", guiCommon),
		dataProvider:       dataProvider,
		selectedContextKey: 0,
		contextKeys:        []string{},
		contentViewport:    &ContextViewport{},
		internalViews:      make(map[string]*gocui.View),
		onClose:            onClose,
		isVisible:          false,
	}

	// Set up internal layout using boxlayout (dual-panel like previous TUI)
	component.setupInternalLayout()

	return component
}

// setupInternalLayout configures a dual-panel layout:
// - Left: Context keys list with arrow selection indicators
// - Right: Context content with scrolling support
// - Bottom: Navigation tips panel
func (c *LLMContextViewerComponent) setupInternalLayout() {
	layout := &boxlayout.Box{
		Direction: boxlayout.ROW, // Vertical split (top to bottom)
		Children: []*boxlayout.Box{
			{
				Direction: boxlayout.COLUMN, // Horizontal split for main content
				Weight:    1,                // Takes most space
				Children: []*boxlayout.Box{
					{
						Window: "context-keys", // Left panel: context keys list
						Size:   25,             // Fixed width for keys (slightly wider than help dialog)
					},
					{
						Window: "context-content", // Right panel: context content
						Weight: 1,                 // Takes remaining horizontal space
					},
				},
			},
			{
				Window: "navigation-tips", // Bottom panel: navigation tips
				Size:   1,                 // Single line like status bar
			},
		},
	}

	c.internalLayout = layout
}

// LoadContextData fetches context data from the controller
func (c *LLMContextViewerComponent) LoadContextData() error {
	contextParts := c.dataProvider.GetContextData()
	if contextParts == nil {
		return fmt.Errorf("failed to load context: no data available")
	}

	// Update context keys and sort them alphabetically
	c.contextKeys = make([]string, 0, len(contextParts))
	for key := range contextParts {
		c.contextKeys = append(c.contextKeys, key)
	}
	sort.Strings(c.contextKeys)

	// Reset selection if we have keys
	if len(c.contextKeys) > 0 {
		c.selectedContextKey = 0
		c.contentViewport.offsetY = 0 // Reset scroll position
	}

	return nil
}

func (c *LLMContextViewerComponent) GetKeybindings() []*types.KeyBinding {
	// Note: Esc and q are handled globally by the app when contextViewerActive is true
	var keybindings []*types.KeyBinding

	// Add context viewer specific keybindings for main dialog view
	contextBindings := []*types.KeyBinding{
		// Navigation in left panel (context keys)
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowUp,
			Mod:     gocui.ModNone,
			Handler: c.handleUp,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowDown,
			Mod:     gocui.ModNone,
			Handler: c.handleDown,
		},
		{
			View:    c.viewName,
			Key:     'k',
			Mod:     gocui.ModNone,
			Handler: c.handleUp,
		},
		{
			View:    c.viewName,
			Key:     'j',
			Mod:     gocui.ModNone,
			Handler: c.handleDown,
		},
		// Content scrolling (right panel)
		{
			View:    c.viewName,
			Key:     gocui.KeyPgup,
			Mod:     gocui.ModNone,
			Handler: c.handlePageUp,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyPgdn,
			Mod:     gocui.ModNone,
			Handler: c.handlePageDown,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlU,
			Mod:     gocui.ModNone,
			Handler: c.handlePageUp,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlD,
			Mod:     gocui.ModNone,
			Handler: c.handlePageDown,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyHome,
			Mod:     gocui.ModNone,
			Handler: c.handleHome,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEnd,
			Mod:     gocui.ModNone,
			Handler: c.handleEnd,
		},
		// Refresh context data
		{
			View:    c.viewName,
			Key:     'r',
			Mod:     gocui.ModNone,
			Handler: c.handleRefresh,
		},
	}

	// Also bind to internal views for better focus handling
	internalViews := []string{
		c.getInternalViewName("context-keys"),
		c.getInternalViewName("context-content"),
		c.getInternalViewName("navigation-tips"),
	}

	for _, viewName := range internalViews {
		contextBindings = append(contextBindings, []*types.KeyBinding{
			{
				View:    viewName,
				Key:     gocui.KeyArrowUp,
				Mod:     gocui.ModNone,
				Handler: c.handleUp,
			},
			{
				View:    viewName,
				Key:     gocui.KeyArrowDown,
				Mod:     gocui.ModNone,
				Handler: c.handleDown,
			},
			{
				View:    viewName,
				Key:     'k',
				Mod:     gocui.ModNone,
				Handler: c.handleUp,
			},
			{
				View:    viewName,
				Key:     'j',
				Mod:     gocui.ModNone,
				Handler: c.handleDown,
			},
			{
				View:    viewName,
				Key:     gocui.KeyPgup,
				Mod:     gocui.ModNone,
				Handler: c.handlePageUp,
			},
			{
				View:    viewName,
				Key:     gocui.KeyPgdn,
				Mod:     gocui.ModNone,
				Handler: c.handlePageDown,
			},
			{
				View:    viewName,
				Key:     gocui.KeyCtrlU,
				Mod:     gocui.ModNone,
				Handler: c.handlePageUp,
			},
			{
				View:    viewName,
				Key:     gocui.KeyCtrlD,
				Mod:     gocui.ModNone,
				Handler: c.handlePageDown,
			},
			{
				View:    viewName,
				Key:     gocui.KeyHome,
				Mod:     gocui.ModNone,
				Handler: c.handleHome,
			},
			{
				View:    viewName,
				Key:     gocui.KeyEnd,
				Mod:     gocui.ModNone,
				Handler: c.handleEnd,
			},
			{
				View:    viewName,
				Key:     'r',
				Mod:     gocui.ModNone,
				Handler: c.handleRefresh,
			},
			{
				View:    viewName,
				Key:     gocui.KeyEsc,
				Mod:     gocui.ModNone,
				Handler: func(g *gocui.Gui, v *gocui.View) error { return c.dataProvider.HandleComponentEvent("close", nil) },
			},
			{
				View:    viewName,
				Key:     'q',
				Mod:     gocui.ModNone,
				Handler: func(g *gocui.Gui, v *gocui.View) error { return c.dataProvider.HandleComponentEvent("close", nil) },
			},
		}...)
	}

	return append(keybindings, contextBindings...)
}

// Navigation handlers for context keys (left panel)
func (c *LLMContextViewerComponent) handleUp(g *gocui.Gui, v *gocui.View) error {
	if c.selectedContextKey > 0 {
		c.selectedContextKey--
		c.contentViewport.offsetY = 0 // Reset scroll when switching context
		return c.Render()
	}
	return nil
}

func (c *LLMContextViewerComponent) handleDown(g *gocui.Gui, v *gocui.View) error {
	if c.selectedContextKey < len(c.contextKeys)-1 {
		c.selectedContextKey++
		c.contentViewport.offsetY = 0 // Reset scroll when switching context
		return c.Render()
	}
	return nil
}

// Content scrolling handlers (right panel)
func (c *LLMContextViewerComponent) handlePageUp(g *gocui.Gui, v *gocui.View) error {
	c.contentViewport.offsetY -= c.contentViewport.viewHeight
	if c.contentViewport.offsetY < 0 {
		c.contentViewport.offsetY = 0
	}
	return c.Render()
}

func (c *LLMContextViewerComponent) handlePageDown(g *gocui.Gui, v *gocui.View) error {
	c.contentViewport.offsetY += c.contentViewport.viewHeight
	if c.contentViewport.offsetY > c.contentViewport.maxY {
		c.contentViewport.offsetY = c.contentViewport.maxY
	}
	return c.Render()
}

func (c *LLMContextViewerComponent) handleHome(g *gocui.Gui, v *gocui.View) error {
	c.contentViewport.offsetY = 0
	return c.Render()
}

func (c *LLMContextViewerComponent) handleEnd(g *gocui.Gui, v *gocui.View) error {
	c.contentViewport.offsetY = c.contentViewport.maxY
	return c.Render()
}

func (c *LLMContextViewerComponent) handleRefresh(g *gocui.Gui, v *gocui.View) error {
	// Request refresh from controller
	if err := c.dataProvider.HandleComponentEvent("refresh", nil); err != nil {
		// Could show error in a status line, for now just ignore
		return nil
	}
	return nil
}

func (c *LLMContextViewerComponent) getInternalViewName(windowName string) string {
	return c.viewName + "-" + windowName
}

// Show displays the context viewer in full-screen mode
func (c *LLMContextViewerComponent) Show() error {
	// Load context data before showing
	if err := c.LoadContextData(); err != nil {
		return fmt.Errorf("failed to load context data: %w", err)
	}

	c.isVisible = true

	// Layout the views
	if err := c.Layout(); err != nil {
		return err
	}

	// Hide cursor globally for clean appearance
	gui := c.BaseComponent.gui.GetGui()
	gui.Update(func(g *gocui.Gui) error {
		g.Cursor = false // Disable cursor globally while viewer is open
		return nil
	})

	return nil
}

// Close restores the cursor and closes the context viewer
func (c *LLMContextViewerComponent) Close() error {
	if !c.isVisible {
		return nil
	}

	c.isVisible = false

	// Clean up views
	gui := c.BaseComponent.gui.GetGui()
	for _, view := range c.internalViews {
		gui.DeleteView(view.Name())
	}
	c.internalViews = make(map[string]*gocui.View)

	// Restore cursor for normal application use
	gui.Update(func(g *gocui.Gui) error {
		g.Cursor = true // Re-enable cursor globally
		return nil
	})

	// Call close callback
	if c.onClose != nil {
		return c.onClose()
	}

	return nil
}

func (c *LLMContextViewerComponent) Render() error {
	// Render left panel (context keys)
	if err := c.renderContextKeysPanel(); err != nil {
		return err
	}

	// Render right panel (context content)
	if err := c.renderContextContentPanel(); err != nil {
		return err
	}

	// Render bottom panel (navigation tips)
	if err := c.renderNavigationTipsPanel(); err != nil {
		return err
	}

	return nil
}

func (c *LLMContextViewerComponent) renderContextKeysPanel() error {
	view := c.GetInternalView("context-keys")
	if view == nil {
		return nil
	}

	view.Clear()
	view.Highlight = false // Disable highlighting
	view.Editable = false  // Disable editing
	view.Title = " Context Parts "

	if len(c.contextKeys) == 0 {
		fmt.Fprintln(view, "  No context data available")
		fmt.Fprintln(view, "  Press 'r' to refresh")
		return nil
	}

	// Render context keys with arrow indicator
	for i, key := range c.contextKeys {
		if i == c.selectedContextKey {
			fmt.Fprintf(view, "► %-20s\n", key)
		} else {
			fmt.Fprintf(view, "  %-20s\n", key)
		}
	}

	return nil
}

func (c *LLMContextViewerComponent) renderContextContentPanel() error {
	view := c.GetInternalView("context-content")
	if view == nil {
		return nil
	}

	view.Clear()
	view.Highlight = false // Disable highlighting
	view.Editable = false  // No cursor needed
	view.Frame = true      // No frame like status bar

	if len(c.contextKeys) == 0 {
		view.Title = " No Content "
		theme := c.gui.GetTheme()
		textColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
		if textColor != "" {
			fmt.Fprintf(view, "%sNo context data available.%s\n", textColor, "\033[0m")
			fmt.Fprintf(view, "%sPress 'r' to refresh context data.%s\n", textColor, "\033[0m")
		} else {
			fmt.Fprintln(view, "No context data available.")
			fmt.Fprintln(view, "Press 'r' to refresh context data.")
		}
		return nil
	}

	selectedKey := c.contextKeys[c.selectedContextKey]
	contextParts := c.dataProvider.GetContextData()
	content, exists := contextParts[selectedKey]

	view.Title = fmt.Sprintf(" {%s} ", selectedKey)

	if !exists || content == "" {
		theme := c.gui.GetTheme()
		textColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
		if textColor != "" {
			fmt.Fprintf(view, "%sNo content available for '%s'%s", textColor, selectedKey, "\033[0m")
		} else {
			fmt.Fprintf(view, "No content available for '%s'", selectedKey)
		}
		return nil
	}

	// Update viewport dimensions
	_, viewHeight := view.Size()
	c.contentViewport.viewHeight = viewHeight

	// Split content into lines for scrolling
	lines := strings.Split(content, "\n")
	c.contentViewport.maxY = len(lines) - viewHeight
	if c.contentViewport.maxY < 0 {
		c.contentViewport.maxY = 0
	}

	// Render visible lines based on viewport offset
	startLine := c.contentViewport.offsetY
	endLine := startLine + viewHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Apply tertiary text color for content
	theme := c.gui.GetTheme()
	textColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
	resetColor := "\033[0m"

	for i := startLine; i < endLine; i++ {
		if textColor != "" {
			fmt.Fprintf(view, "%s%s%s\n", textColor, lines[i], resetColor)
		} else {
			fmt.Fprintln(view, lines[i])
		}
	}

	return nil
}

func (c *LLMContextViewerComponent) renderNavigationTipsPanel() error {
	view := c.GetInternalView("navigation-tips")
	if view == nil {
		return nil
	}

	view.Clear()
	view.Highlight = false // Disable highlighting
	view.Editable = false  // No cursor needed
	view.Frame = false     // No frame like status bar
	view.Title = ""        // No title needed

	// Simple navigation instructions with left padding like status bar, using secondary color
	text := "↑↓ Navigate | PgUp/PgDn Scroll | Home/End Jump | r Refresh | Esc/q Close"
	text = " " + text // Add left padding like status bar

	// Apply secondary color for system UI elements
	theme := c.gui.GetTheme()
	secondaryColor := presentation.ConvertColorToAnsi(theme.Secondary)
	if secondaryColor != "" {
		text = secondaryColor + text + "\033[0m" // Reset color after text
	}

	fmt.Fprint(view, text)

	return nil
}

// Layout creates the full-screen layout for the context viewer
func (c *LLMContextViewerComponent) Layout() error {
	gui := c.BaseComponent.gui.GetGui()
	maxX, maxY := gui.Size()

	// Initialize internalViews if needed
	if c.internalViews == nil {
		c.internalViews = make(map[string]*gocui.View)
	}

	// Let boxlayout calculate positions for full screen
	windowDimensions := boxlayout.ArrangeWindows(
		c.internalLayout,
		0, 0,
		maxX, maxY,
	)

	// Create views for each window
	for windowName, dims := range windowDimensions {
		viewName := c.getInternalViewName(windowName)

		// For navigation-tips, apply frame offset for status-bar-like positioning
		if windowName == "navigation-tips" {
			frameOffset := 1
			view, err := gui.SetView(viewName, dims.X0-frameOffset, dims.Y0-frameOffset, dims.X1+frameOffset, dims.Y1+frameOffset, 0)
			if err != nil && err != gocui.ErrUnknownView {
				return err
			}
			if view != nil {
				view.Frame = false
				view.Wrap = false
				view.Autoscroll = false
				view.Highlight = false
				view.Editable = false
				c.internalViews[windowName] = view
			}
		} else {
			// For other views, standard positioning with frames
			view, err := gui.SetView(viewName, dims.X0, dims.Y0, dims.X1, dims.Y1, 0)
			if err != nil && err != gocui.ErrUnknownView {
				return err
			}
			if view != nil {
				view.Frame = true
				view.Wrap = false
				view.Autoscroll = false
				view.Highlight = false
				view.Editable = false
				c.internalViews[windowName] = view
			}
		}
	}

	return nil
}

// GetInternalView returns a view by window name
func (c *LLMContextViewerComponent) GetInternalView(windowName string) *gocui.View {
	return c.internalViews[windowName]
}

// IsVisible returns whether the context viewer is currently visible
func (c *LLMContextViewerComponent) IsVisible() bool {
	return c.isVisible
}

// SelectContextKey allows external code to jump to a specific context key
func (c *LLMContextViewerComponent) SelectContextKey(keyName string) {
	for i, key := range c.contextKeys {
		if strings.EqualFold(key, keyName) {
			c.selectedContextKey = i
			c.contentViewport.offsetY = 0 // Reset scroll
			break
		}
	}
}
