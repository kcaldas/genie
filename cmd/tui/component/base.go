package component

import (
	"sync"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type BaseComponent struct {
	key           string
	viewName      string
	windowName    string
	view          *gocui.View
	gui           types.Gui
	configManager *helpers.ConfigManager

	controlledBounds bool

	onFocus     func() error
	onFocusLost func() error

	// UI properties
	title            string
	windowProperties types.WindowProperties
	isKeybindingsSet bool
	
	// Protects title and other UI properties
	mu sync.RWMutex
}

func NewBaseComponent(key, viewName string, gui types.Gui, configManager *helpers.ConfigManager) *BaseComponent {
	return &BaseComponent{
		key:              key,
		viewName:         viewName,
		windowName:       viewName,
		configManager:    configManager,
		gui:              gui,
		controlledBounds: true,
		title:            "",
		windowProperties: types.WindowProperties{
			Focusable:   true,
			Editable:    false,
			Wrap:        true,
			Autoscroll:  false,
			Highlight:   true,
			Frame:       true,
			BorderStyle: types.BorderStyleSingle, // Default to single border
			BorderColor: "",                      // Use theme color
			FocusBorder: true,                    // Show focus border
			FocusStyle:  types.FocusStyleBorder,  // Default to border focus
		},
	}
}

func (c *BaseComponent) GetKey() string {
	return c.key
}

func (c *BaseComponent) GetViewName() string {
	return c.viewName
}

func (c *BaseComponent) GetView() *gocui.View {
	if c.view == nil && c.gui != nil && c.gui.GetGui() != nil {
		c.view, _ = c.gui.GetGui().View(c.viewName)
	}
	return c.view
}

func (c *BaseComponent) SetView(v *gocui.View) {
	c.view = v
}

// GetTheme returns the current theme from ConfigManager
func (c *BaseComponent) GetTheme() *types.Theme {
	return c.configManager.GetTheme()
}

// GetConfig returns the current config from ConfigManager
func (c *BaseComponent) GetConfig() *types.Config {
	return c.configManager.GetConfig()
}

func (c *BaseComponent) HandleFocus() error {
	// Apply theme-aware border colors for focus
	c.applyThemeBorderColors(true)

	if c.onFocus != nil {
		return c.onFocus()
	}
	return nil
}

func (c *BaseComponent) HandleFocusLost() error {
	// Apply theme-aware border colors for unfocused state
	c.applyThemeBorderColors(false)

	if c.onFocusLost != nil {
		return c.onFocusLost()
	}
	return nil
}

func (c *BaseComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{}
}

func (c *BaseComponent) Render() error {
	// Apply initial theme border colors when rendering
	c.applyThemeBorderColors(false) // Start with unfocused state
	return nil
}

func (c *BaseComponent) SetOnFocus(fn func() error) {
	c.onFocus = fn
}

func (c *BaseComponent) SetOnFocusLost(fn func() error) {
	c.onFocusLost = fn
}

func (c *BaseComponent) HasControlledBounds() bool {
	return c.controlledBounds
}

func (c *BaseComponent) SetWindowName(windowName string) {
	c.windowName = windowName
}

func (c *BaseComponent) SetControlledBounds(controlled bool) {
	c.controlledBounds = controlled
}

func (c *BaseComponent) GetWindowProperties() types.WindowProperties {
	return c.windowProperties
}

func (c *BaseComponent) GetTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.title
}

func (c *BaseComponent) SetTitle(title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.title = title
}

func (c *BaseComponent) SetWindowProperties(props types.WindowProperties) {
	c.windowProperties = props
}

// applyThemeBorderColors applies theme-appropriate colors to view borders
// This overrides the global GUI frame colors for this specific component
func (c *BaseComponent) applyThemeBorderColors(focused bool) {
	view := c.GetView()
	if view == nil || !c.windowProperties.Frame {
		return
	}

	theme := c.configManager.GetTheme()
	if theme == nil {
		return
	}

	// Skip border coloring for components that don't want borders
	if c.windowProperties.BorderStyle == types.BorderStyleNone {
		return
	}

	// Get theme and apply border color using single conversion function
	config := c.configManager.GetConfig()
	newTheme := presentation.GetTheme(config.Theme)
	
	// Choose the right border color based on focus state
	var borderHexColor string
	var titleHexColor string
	if focused && c.windowProperties.FocusBorder {
		borderHexColor = newTheme.BorderFocused
		titleHexColor = newTheme.TitleFocused
	} else {
		borderHexColor = newTheme.BorderDefault
		titleHexColor = newTheme.TitleDefault
	}
	
	// Use single conversion function to get gocui.Attribute
	frameColor := presentation.GetThemeColor(borderHexColor)
	titleColor := presentation.GetThemeColor(titleHexColor)
	
	// Apply border and title colors separately
	view.FrameColor = frameColor
	view.TitleColor = titleColor
}

// RefreshThemeColors updates border colors based on current theme
func (c *BaseComponent) RefreshThemeColors() {
	// Apply current border colors (assuming unfocused state)
	c.applyThemeBorderColors(false)
}
