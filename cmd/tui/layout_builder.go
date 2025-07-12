package tui

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
)

// LayoutBuilder helps construct and configure the layout manager
type LayoutBuilder struct {
	layoutManager *layout.LayoutManager
}

// NewLayoutBuilder creates a new layout builder with all components
func NewLayoutBuilder(
	gui *gocui.Gui,
	configManager *helpers.ConfigManager,
	messagesComponent *component.MessagesComponent,
	inputComponent *component.InputComponent,
	statusComponent *component.StatusComponent,
	textViewerComponent *component.TextViewerComponent,
	diffViewerComponent *component.DiffViewerComponent,
	debugComponent *component.DebugComponent,
) *LayoutBuilder {
	// Create layout config and manager
	config := configManager.GetConfig()
	layoutConfig := layout.NewDefaultLayoutConfig(config)
	layoutManager := layout.NewLayoutManager(gui, layoutConfig)
	
	// Create the builder
	builder := &LayoutBuilder{
		layoutManager: layoutManager,
	}
	
	// Setup all components
	builder.setupComponents(
		messagesComponent,
		inputComponent,
		statusComponent,
		textViewerComponent,
		diffViewerComponent,
		debugComponent,
	)
	
	// Setup status sub-components
	builder.setupStatusSubComponents(statusComponent)
	
	return builder
}

// setupComponents maps all components to their panel positions
func (lb *LayoutBuilder) setupComponents(
	messagesComponent *component.MessagesComponent,
	inputComponent *component.InputComponent,
	statusComponent *component.StatusComponent,
	textViewerComponent *component.TextViewerComponent,
	diffViewerComponent *component.DiffViewerComponent,
	debugComponent *component.DebugComponent,
) {
	// Map components using semantic names
	lb.layoutManager.SetComponent("messages", messagesComponent)      // messages in center
	lb.layoutManager.SetComponent("input", inputComponent)            // input at bottom
	lb.layoutManager.SetComponent("text-viewer", textViewerComponent) // text viewer on right side
	lb.layoutManager.SetComponent("diff-viewer", diffViewerComponent) // diff viewer on right side
	lb.layoutManager.SetComponent("status", statusComponent)          // status at top
	lb.layoutManager.SetComponent("debug", debugComponent)            // debug on right side
}

// setupStatusSubComponents registers the status bar sub-components
func (lb *LayoutBuilder) setupStatusSubComponents(statusComponent *component.StatusComponent) {
	// Register status sub-components
	lb.layoutManager.AddSubPanel("status", "status-left", statusComponent.GetLeftComponent())
	lb.layoutManager.AddSubPanel("status", "status-center", statusComponent.GetCenterComponent())
	lb.layoutManager.AddSubPanel("status", "status-right", statusComponent.GetRightComponent())
}

// GetLayoutManager returns the configured layout manager
func (lb *LayoutBuilder) GetLayoutManager() *layout.LayoutManager {
	return lb.layoutManager
}