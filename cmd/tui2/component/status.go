package component

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/types"
)

type StatusComponent struct {
	*BaseComponent
	stateAccessor   types.IStateAccessor
	leftComponent   *StatusSectionComponent
	centerComponent *StatusSectionComponent
	rightComponent  *StatusSectionComponent
}

type StatusSectionComponent struct {
	*BaseComponent
	text string
}

func NewStatusSectionComponent(name, viewName string, gui types.IGuiCommon) *StatusSectionComponent {
	ctx := &StatusSectionComponent{
		BaseComponent: NewBaseComponent(name, viewName, gui),
		text:          "",
	}

	// Configure section properties - frameless like main status
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:   false,
		Editable:    false,
		Wrap:        false,
		Autoscroll:  false,
		Highlight:   false,
		Frame:       false,
		BorderStyle: types.BorderStyleNone, // Status sections have no borders
		FocusStyle:  types.FocusStyleNone,  // Status sections not focusable
	})

	ctx.SetControlledBounds(true)
	return ctx
}

func (c *StatusSectionComponent) SetText(text string) {
	c.text = text
}

func (c *StatusSectionComponent) Render() error {
	v := c.GetView()
	if v == nil {
		return nil
	}

	v.Clear()
	
	// Add padding based on the component name
	text := c.text
	if c.GetViewName() == "status-left" {
		text = " " + text // Add left padding
	}
	
	fmt.Fprint(v, text)
	return nil
}

func NewStatusComponent(gui types.IGuiCommon, state types.IStateAccessor) *StatusComponent {
	ctx := &StatusComponent{
		BaseComponent:   NewBaseComponent("status", "status", gui),
		stateAccessor:   state,
		leftComponent:   NewStatusSectionComponent("status-left", "status-left", gui),
		centerComponent: NewStatusSectionComponent("status-center", "status-center", gui),
		rightComponent:  NewStatusSectionComponent("status-right", "status-right", gui),
	}

	// Configure StatusComponent specific properties
	ctx.SetTitle(" Status ")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:   false,
		Editable:    false,
		Wrap:        false,
		Autoscroll:  false,
		Highlight:   false,
		Frame:       false,
		BorderStyle: types.BorderStyleNone, // Status bar has no borders
		FocusStyle:  types.FocusStyleNone,  // Status bar not focusable
	})

	ctx.SetWindowName("status")
	ctx.SetControlledBounds(true)

	return ctx
}

// SetLeftText sets the text to display on the left side of the status bar
func (c *StatusComponent) SetLeftText(text string) {
	c.leftComponent.SetText(text)
}

// SetCenterText sets the text to display in the center of the status bar
func (c *StatusComponent) SetCenterText(text string) {
	c.centerComponent.SetText(text)
}

// SetRightText sets the text to display on the right side of the status bar
func (c *StatusComponent) SetRightText(text string) {
	c.rightComponent.SetText(text)
}

// SetStatusTexts sets all three text sections at once
func (c *StatusComponent) SetStatusTexts(left, center, right string) {
	c.leftComponent.SetText(left)
	c.centerComponent.SetText(center)
	c.rightComponent.SetText(right)
}

// GetLeftComponent returns the left section component
func (c *StatusComponent) GetLeftComponent() *StatusSectionComponent {
	return c.leftComponent
}

// GetCenterComponent returns the center section component
func (c *StatusComponent) GetCenterComponent() *StatusSectionComponent {
	return c.centerComponent
}

// GetRightComponent returns the right section component
func (c *StatusComponent) GetRightComponent() *StatusSectionComponent {
	return c.rightComponent
}

func (c *StatusComponent) Render() error {
	// Set default content if sections are empty
	if c.leftComponent.text == "" {
		c.leftComponent.SetText("Ready")
	}

	// Set center text based on debug status (only if not already set)
	config := c.gui.GetConfig()
	if config.DebugEnabled {
		// Apply secondary color to debug status
		theme := c.gui.GetTheme()
		secondaryColor := presentation.ConvertColorToAnsi(theme.Secondary)
		resetColor := "\033[0m"
		
		centerText := "Debug is ON"
		if secondaryColor != "" {
			centerText = secondaryColor + centerText + resetColor
		}
		c.centerComponent.SetText(centerText)
	} else if c.centerComponent.text == "" || strings.Contains(c.centerComponent.text, "Debug is ON") {
		// Only clear if it's empty or was showing debug status
		c.centerComponent.SetText("")
	}

	// Only update right text with stats if no custom text is set
	if c.rightComponent.text == "" {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memMB := m.Alloc / 1024 / 1024
		msgCount := len(c.stateAccessor.GetMessages())
		rightText := fmt.Sprintf("Messages: %d | Memory: %dMB", msgCount, memMB)
		c.rightComponent.SetText(rightText)
	}

	// Render each section component
	if err := c.leftComponent.Render(); err != nil {
		return err
	}
	if err := c.centerComponent.Render(); err != nil {
		return err
	}
	if err := c.rightComponent.Render(); err != nil {
		return err
	}

	return nil
}
