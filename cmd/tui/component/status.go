package component

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type StatusComponent struct {
	*BaseComponent
	stateAccessor   types.IStateAccessor
	configManager   *helpers.ConfigManager
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

func NewStatusComponent(gui types.IGuiCommon, state types.IStateAccessor, configManager *helpers.ConfigManager, eventBus *events.CommandEventBus) *StatusComponent {
	ctx := &StatusComponent{
		BaseComponent:   NewBaseComponent("status", "status", gui),
		stateAccessor:   state,
		configManager:   configManager,
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

	// Subscribe to theme changes
	eventBus.Subscribe("theme.changed", func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.Render()
		})
	})

	return ctx
}

func (c *StatusComponent) StartStatusUpdates() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond) // Update 10 times per second for smooth spinner
		defer ticker.Stop()

		for range ticker.C {
			c.gui.PostUIUpdate(func() {
				c.Render()
			})
		}
	}()
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

// SetLeftToReady sets the left text to the ready indicator (colored circle)
func (c *StatusComponent) SetLeftToReady() {
	c.leftComponent.SetText(c.getReadyIndicator())
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

// getReadyIndicator returns a colored circle to indicate ready state
func (c *StatusComponent) getReadyIndicator() string {
	theme := c.gui.GetTheme()
	primaryColor := presentation.ConvertColorToAnsi(theme.Primary)
	resetColor := "\033[0m"

	circle := "Ready" //"●" Filled circle (U+25CF)
	if primaryColor != "" {
		circle = primaryColor + circle + resetColor
	}
	return circle
}

func (c *StatusComponent) getSpinnerFrame() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := frames[time.Now().UnixNano()/100000000%int64(len(frames))]

	// Color the spinner with error color
	config := c.configManager.GetConfig()
	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
	errorColor := presentation.ConvertColorToAnsi(theme.Error)
	resetColor := "\033[0m"

	if errorColor != "" {
		frame = errorColor + frame + resetColor
	}
	return frame
}

func (c *StatusComponent) getConfirmationSpinnerFrame() string {
	frames := []string{"◐", "◓", "◑", "◒"}
	frame := frames[time.Now().UnixNano()/200000000%int64(len(frames))]

	// Color the confirmation spinner with error color
	config := c.configManager.GetConfig()
	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
	errorColor := presentation.ConvertColorToAnsi(theme.Error)
	resetColor := "\033[0m"

	if errorColor != "" {
		frame = errorColor + frame + resetColor
	}
	return frame
}

// getThinkingText returns "Thinking" text with optional time in tertiary color
func (c *StatusComponent) getThinkingText(seconds *int) string {
	config := c.configManager.GetConfig()
	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
	tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
	resetColor := "\033[0m"

	thinkingText := "Thinking"
	if tertiaryColor != "" {
		thinkingText = tertiaryColor + thinkingText + resetColor
	}

	if seconds != nil {
		timeText := fmt.Sprintf("(%ds)", *seconds)
		if tertiaryColor != "" {
			timeText = tertiaryColor + timeText + resetColor
		}
		return fmt.Sprintf("%s %s", thinkingText, timeText)
	}

	return thinkingText
}

func (c *StatusComponent) Render() error {
	// Update spinner based on current state - confirmation takes priority
	if c.stateAccessor.IsWaitingConfirmation() {
		spinner := c.getConfirmationSpinnerFrame()
		c.SetLeftText("Your call " + spinner)
	} else if c.stateAccessor.IsLoading() {
		spinner := c.getSpinnerFrame()
		duration := c.stateAccessor.GetLoadingDuration()
		seconds := int(duration.Seconds())
		thinkingText := c.getThinkingText(&seconds)
		config := c.configManager.GetConfig()
		theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
		tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
		resetColor := "\033[0m"
		escHint := "(ESC to cancel)"
		if tertiaryColor != "" {
			escHint = tertiaryColor + escHint + resetColor
		}
		c.leftComponent.SetText(fmt.Sprintf("%s %s %s", thinkingText, spinner, escHint))
	} else if !c.stateAccessor.IsLoading() {
		c.leftComponent.SetText(c.getReadyIndicator())
	}

	// Set default content if sections are empty
	if c.leftComponent.text == "" {
		c.leftComponent.SetText(c.getReadyIndicator())
	}

	// Set center text based on debug status (only if not already set)
	config := c.configManager.GetConfig()
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

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memMB := m.Alloc / 1024 / 1024
	msgCount := len(c.stateAccessor.GetMessages())

	// Apply tertiary color to the stats text
	theme := c.gui.GetTheme()
	tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
	resetColor := "\033[0m"

	rightText := fmt.Sprintf("Messages: %d | Memory: %dMB", msgCount, memMB)
	if tertiaryColor != "" {
		rightText = tertiaryColor + rightText + resetColor
	}
	c.rightComponent.SetText(rightText)

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
