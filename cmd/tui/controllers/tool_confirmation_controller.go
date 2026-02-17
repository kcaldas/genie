package controllers

import (
	"fmt"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	core_events "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/logging"
)

type ToolConfirmationController struct {
	*ConfirmationKeyHandler
	gui                   types.Gui
	stateAccessor         types.IStateAccessor
	layoutManager         *layout.LayoutManager
	inputComponent        types.Component
	configManager         *helpers.ConfigManager
	ConfirmationComponent *component.ConfirmationComponent
	textViewerComponent   *component.TextViewerComponent
	eventBus              core_events.EventBus
	commandEventBus       *events.CommandEventBus
}

func NewToolConfirmationController(
	gui types.Gui,
	stateAccessor types.IStateAccessor,
	layoutManager *layout.LayoutManager,
	inputComponent types.Component,
	textViewerComponent *component.TextViewerComponent,
	configManager *helpers.ConfigManager,
	eventBus core_events.EventBus,
	commandEventBus *events.CommandEventBus,
) *ToolConfirmationController {
	c := ToolConfirmationController{
		ConfirmationKeyHandler: NewConfirmationKeyHandler(),
		gui:                    gui,
		stateAccessor:          stateAccessor,
		layoutManager:          layoutManager,
		inputComponent:         inputComponent,
		textViewerComponent:    textViewerComponent,
		configManager:          configManager,
		eventBus:               eventBus,
		commandEventBus:        commandEventBus,
	}

	eventBus.Subscribe("tool.confirmation.request", func(e interface{}) {
		if event, ok := e.(core_events.ToolConfirmationRequest); ok {
			logging.GetGlobalLogger().Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			c.HandleToolConfirmationRequest(event)
		}
	})

	// Subscribe to user cancel input
	commandEventBus.Subscribe("user.input.cancel", func(event interface{}) {
		// Skip if no confirmation is active
		if c.ConfirmationComponent == nil {
			return
		}
		c.stateAccessor.SetWaitingConfirmation(false)
		c.ConfirmationComponent = nil
		// All gocui state modifications must run on the main loop
		c.gui.GetGui().Update(func(g *gocui.Gui) error {
			c.layoutManager.HideRightPanel()
			c.layoutManager.SwapComponent("input", c.inputComponent)
			if err := c.inputComponent.Render(); err != nil {
				return err
			}
			return c.focusPanelByName("input")
		})
	})

	return &c
}

// logger returns the current global logger (updated dynamically when debug is toggled)
func (tc *ToolConfirmationController) logger() logging.Logger {
	return logging.GetGlobalLogger()
}

func (tc *ToolConfirmationController) HandleToolConfirmationRequest(event core_events.ToolConfirmationRequest) error {
	// Check if tool has auto-accept enabled
	config := tc.configManager.GetConfig()
	if toolConfig, exists := config.ToolConfigs[event.ToolName]; exists && toolConfig.AutoAccept {
		// Auto-accept without showing dialog
		tc.logger().Debug(fmt.Sprintf("Auto-accepting confirmation for tool: %s", event.ToolName))
		tc.eventBus.Publish("tool.confirmation.response", core_events.ToolConfirmationResponse{
			ExecutionID: event.ExecutionID,
			Confirmed:   true,
		})
		return nil
	}

	// Set confirmation state
	tc.stateAccessor.SetWaitingConfirmation(true)

	// Always create a new confirmation component for tool confirmations
	tc.ConfirmationComponent = component.NewConfirmationComponent(
		tc.gui,
		tc.configManager,
		event.ExecutionID,
		"1 - Yes | 2 - No",
		tc.HandleToolConfirmationResponse, // Connect to controller's response handler
	)

	title := fmt.Sprintf("Tool: %s", event.ToolName)
	message := event.Message
	tc.logger().Debug("Showing confirmation message in viewer", "message", message, "tool", event.ToolName)

	// All gocui state modifications must run on the main loop
	tc.gui.GetGui().Update(func(g *gocui.Gui) error {
		// Swap to confirmation component
		tc.layoutManager.SwapComponent("input", tc.ConfirmationComponent)

		// Apply secondary theme color to border and title
		if view, err := g.View("input"); err == nil {
			theme := tc.configManager.GetTheme()
			if theme != nil {
				secondaryColor := presentation.ConvertAnsiToGocuiColor(theme.Secondary)
				view.FrameColor = secondaryColor
				view.TitleColor = secondaryColor
			}
		}

		if err := tc.ConfirmationComponent.Render(); err != nil {
			tc.logger().Debug("Failed to render confirmation component", "error", err)
		}

		// Focus the confirmation component
		tc.focusPanelByName("input")

		// Show the confirmation message in the text-viewer panel
		tc.layoutManager.ShowRightPanel("text-viewer")
		tc.textViewerComponent.SetContentWithType(message, "text")
		tc.textViewerComponent.SetTitle(title)

		return nil
	})

	// Queue a follow-up render for the viewer (needs view to exist after Layout)
	tc.gui.PostUIUpdate(func() {
		if view, err := tc.gui.GetGui().View("text-viewer"); err == nil && view != nil {
			tc.textViewerComponent.Render()
		}
	})

	return nil
}

// HandleKeyPress processes a key press and determines if it's a confirmation response
func (tc *ToolConfirmationController) HandleKeyPress(key interface{}) (bool, error) {
	// Check if we have an active confirmation
	if tc.ConfirmationComponent == nil {
		return false, nil
	}

	// Use the embedded key handler to interpret the key
	confirmed, handled := tc.InterpretKey(key)
	if handled {
		executionID := tc.ConfirmationComponent.ExecutionID
		return true, tc.HandleToolConfirmationResponse(executionID, confirmed)
	}

	return false, nil
}

func (tc *ToolConfirmationController) HandleToolConfirmationResponse(executionID string, confirmed bool) error {
	// Clear confirmation state
	tc.stateAccessor.SetWaitingConfirmation(false)
	tc.ConfirmationComponent = nil

	// Publish confirmation response
	tc.logger().Debug(fmt.Sprintf("Event published: tool.confirmation.response (confirmed=%v)", confirmed))
	tc.eventBus.Publish("tool.confirmation.response", core_events.ToolConfirmationResponse{
		ExecutionID: executionID,
		Confirmed:   confirmed,
	})

	// All gocui state modifications must run on the main loop
	tc.gui.GetGui().Update(func(g *gocui.Gui) error {
		tc.layoutManager.HideRightPanel()
		tc.layoutManager.SwapComponent("input", tc.inputComponent)
		if err := tc.inputComponent.Render(); err != nil {
			return err
		}
		return tc.focusPanelByName("input")
	})

	return nil
}

func (tc *ToolConfirmationController) focusPanelByName(panelName string) error {
	// Delegate to layout manager for panel focusing
	if err := tc.layoutManager.FocusPanel(panelName); err != nil {
		return err
	}
	return nil
}
