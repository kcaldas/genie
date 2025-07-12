package layout

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// LayoutBuilder helps construct and configure the layout manager
type LayoutBuilder struct {
	configManager *helpers.ConfigManager
	layoutManager *LayoutManager
}

// NewLayoutBuilder creates a new layout builder
func NewLayoutBuilder(gui *gocui.Gui, configManager *helpers.ConfigManager) *LayoutBuilder {
	config := configManager.GetConfig()
	layoutConfig := NewDefaultLayoutConfig(config)
	layoutManager := NewLayoutManager(gui, layoutConfig)
	
	return &LayoutBuilder{
		configManager: configManager,
		layoutManager: layoutManager,
	}
}

// SetupComponents maps all components to their panel positions
func (lb *LayoutBuilder) SetupComponents(
	messagesComponent types.Component,
	inputComponent types.Component,
	statusComponent types.Component,
	textViewerComponent types.Component,
	diffViewerComponent types.Component,
	debugComponent types.Component,
) {
	// Map components using semantic names
	lb.layoutManager.SetComponent("messages", messagesComponent)      // messages in center
	lb.layoutManager.SetComponent("input", inputComponent)            // input at bottom
	lb.layoutManager.SetComponent("text-viewer", textViewerComponent) // text viewer on right side
	lb.layoutManager.SetComponent("diff-viewer", diffViewerComponent) // diff viewer on right side
	lb.layoutManager.SetComponent("status", statusComponent)          // status at top
	lb.layoutManager.SetComponent("debug", debugComponent)            // debug on right side
}

// SetupStatusSubComponents registers the status bar sub-components
func (lb *LayoutBuilder) SetupStatusSubComponents(statusComponent interface {
	GetLeftComponent() types.Component
	GetCenterComponent() types.Component
	GetRightComponent() types.Component
}) {
	// Register status sub-components
	lb.layoutManager.AddSubPanel("status", "status-left", statusComponent.GetLeftComponent())
	lb.layoutManager.AddSubPanel("status", "status-center", statusComponent.GetCenterComponent())
	lb.layoutManager.AddSubPanel("status", "status-right", statusComponent.GetRightComponent())
}

// Build returns the configured layout manager
func (lb *LayoutBuilder) Build() *LayoutManager {
	return lb.layoutManager
}

// GetLayoutManager returns the layout manager (convenience method)
func (lb *LayoutBuilder) GetLayoutManager() *LayoutManager {
	return lb.layoutManager
}