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
)

type ToolConfirmationController struct {
	*ConfirmationKeyHandler
	gui                   types.Gui
	stateAccessor         types.IStateAccessor
	layoutManager         *layout.LayoutManager
	inputComponent        types.Component
	configManager         *helpers.ConfigManager
	ConfirmationComponent *component.ConfirmationComponent
	eventBus              core_events.EventBus
	commandEventBus       *events.CommandEventBus
	logger                types.Logger
}

func NewToolConfirmationController(
	gui types.Gui,
	stateAccessor types.IStateAccessor,
	layoutManager *layout.LayoutManager,
	inputComponent types.Component,
	configManager *helpers.ConfigManager,
	eventBus core_events.EventBus,
	commandEventBus *events.CommandEventBus,
	logger types.Logger,
) *ToolConfirmationController {
	c := ToolConfirmationController{
		ConfirmationKeyHandler: NewConfirmationKeyHandler(),
		gui:                    gui,
		stateAccessor:          stateAccessor,
		layoutManager:          layoutManager,
		inputComponent:         inputComponent,
		configManager:          configManager,
		eventBus:               eventBus,
		commandEventBus:        commandEventBus,
		logger:                 logger,
	}

	eventBus.Subscribe("tool.confirmation.request", func(e interface{}) {
		if event, ok := e.(core_events.ToolConfirmationRequest); ok {
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			c.HandleToolConfirmationRequest(event)
		}
	})

	// Subscribe to user cancel input
	commandEventBus.Subscribe("user.input.cancel", func(event interface{}) {
		c.stateAccessor.SetWaitingConfirmation(false)
		c.layoutManager.SwapComponent("input", c.inputComponent)
		// Re-render to update the view
		c.gui.GetGui().Update(func(g *gocui.Gui) error {
			if err := c.inputComponent.Render(); err != nil {
				return err
			}
			// Focus back on input
			return c.focusPanelByName("input")
		})
	})

	return &c
}

func (tc *ToolConfirmationController) HandleToolConfirmationRequest(event core_events.ToolConfirmationRequest) error {
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

	// Set the active confirmation type so global keybindings route correctly
	tc.stateAccessor.SetActiveConfirmationType("tool")

	// Swap to confirmation component
	tc.layoutManager.SwapComponent("input", tc.ConfirmationComponent)

	// Apply secondary theme color to border and title after swap
	tc.gui.GetGui().Update(func(g *gocui.Gui) error {
		if view, err := g.View("input"); err == nil {
			// Get secondary color from theme
			theme := tc.configManager.GetTheme()
			if theme != nil {
				// Convert secondary color to gocui color for border and title
				secondaryColor := presentation.ConvertAnsiToGocuiColor(theme.Secondary)
				view.FrameColor = secondaryColor
				view.TitleColor = secondaryColor
			}
		}
		return nil
	})

	tc.gui.PostUIUpdate(func() {
		// Render messages first
		if err := tc.ConfirmationComponent.Render(); err != nil {
			// TODO handle error
		}
	})

	// Focus the confirmation component (same view name as input)
	return tc.focusPanelByName("input")
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

	// Publish confirmation response
	tc.logger.Debug(fmt.Sprintf("Event published: tool.confirmation.response (confirmed=%v)", confirmed))
	tc.eventBus.Publish("tool.confirmation.response", core_events.ToolConfirmationResponse{
		ExecutionID: executionID,
		Confirmed:   confirmed,
	})

	// Swap back to input component
	tc.layoutManager.SwapComponent("input", tc.inputComponent)

	// Re-render to update the view
	tc.gui.GetGui().Update(func(g *gocui.Gui) error {
		if err := tc.inputComponent.Render(); err != nil {
			return err
		}
		// Focus back on input
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
