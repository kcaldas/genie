package controllers

import (
	"time"

	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// HelpRenderer interface for generating help content
type HelpRenderer interface {
	RenderHelp() string
}

// HelpControllerInterface defines the interface for help operations
type HelpControllerInterface interface {
	ShowHelp() error
	ToggleHelp() error
	IsVisible() bool
}

// HelpController manages help display functionality
type HelpController struct {
	*BaseController
	layoutManager       *layout.LayoutManager
	textViewerComponent *component.TextViewerComponent
	helpRenderer        HelpRenderer
	helpText            string // Cache help text
}

// NewHelpController creates a new help controller
func NewHelpController(
	gui types.Gui,
	layoutManager *layout.LayoutManager,
	textViewerComponent *component.TextViewerComponent,
	helpRenderer HelpRenderer,
	configManager *helpers.ConfigManager,
) *HelpController {
	return &HelpController{
		BaseController:      NewBaseController(nil, gui, configManager), // No specific component for help controller
		layoutManager:       layoutManager,
		textViewerComponent: textViewerComponent,
		helpRenderer:        helpRenderer,
	}
}

// ShowHelp displays help content in the text viewer panel
func (c *HelpController) ShowHelp() error {
	// Show the right panel in text-viewer mode
	c.layoutManager.ShowRightPanel("text-viewer")

	// Generate help text if not cached
	if c.helpText == "" {
		c.helpText = c.helpRenderer.RenderHelp()
	}

	// Set content in text viewer
	c.textViewerComponent.SetContentWithType(c.helpText, "markdown")
	c.textViewerComponent.SetTitle("Help")

	// Small delay to ensure proper rendering
	time.Sleep(50 * time.Millisecond)

	// Ensure the text viewer renders with the new content
	c.PostUIUpdate(func() {
		c.textViewerComponent.Render()
	})

	return nil
}

// ToggleHelp toggles help visibility
func (c *HelpController) ToggleHelp() error {
	// If right panel is visible and showing text-viewer, hide it
	if c.layoutManager.IsRightPanelVisible() && c.layoutManager.GetRightPanelMode() == "text-viewer" {
		c.layoutManager.HideRightPanel()
		return nil
	}

	// Otherwise show help
	return c.ShowHelp()
}

// IsVisible returns whether help is currently visible
func (c *HelpController) IsVisible() bool {
	return c.layoutManager.IsRightPanelVisible() && c.layoutManager.GetRightPanelMode() == "text-viewer"
}

// RefreshHelp clears the cached help text and regenerates it
func (c *HelpController) RefreshHelp() error {
	c.helpText = "" // Clear cache
	if c.IsVisible() {
		return c.ShowHelp() // Re-show with fresh content
	}
	return nil
}

