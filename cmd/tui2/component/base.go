package component

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type BaseComponent struct {
	key        string
	viewName   string
	windowName string
	view       *gocui.View
	gui        types.IGuiCommon
	
	controlledBounds bool
	transient        bool
	
	onFocus     func() error
	onFocusLost func() error
}

func NewBaseComponent(key, viewName string, gui types.IGuiCommon) *BaseComponent {
	return &BaseComponent{
		key:              key,
		viewName:         viewName,
		windowName:       viewName,
		gui:              gui,
		controlledBounds: true,
		transient:        false,
	}
}

func (c *BaseComponent) GetKey() string {
	return c.key
}

func (c *BaseComponent) GetViewName() string {
	return c.viewName
}

func (c *BaseComponent) GetWindowName() string {
	return c.windowName
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

func (c *BaseComponent) HandleFocus() error {
	if c.onFocus != nil {
		return c.onFocus()
	}
	return nil
}

func (c *BaseComponent) HandleFocusLost() error {
	if c.onFocusLost != nil {
		return c.onFocusLost()
	}
	return nil
}

func (c *BaseComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{}
}

func (c *BaseComponent) Render() error {
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

func (c *BaseComponent) IsTransient() bool {
	return c.transient
}

func (c *BaseComponent) SetWindowName(windowName string) {
	c.windowName = windowName
}

func (c *BaseComponent) SetControlledBounds(controlled bool) {
	c.controlledBounds = controlled
}

func (c *BaseComponent) SetTransient(transient bool) {
	c.transient = transient
}