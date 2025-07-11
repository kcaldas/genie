package controllers

import (
	"fmt"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/events"
	core_events "github.com/kcaldas/genie/pkg/events"
)

type ToolConfirmationController struct {
	*ConfirmationKeyHandler
	gui                       types.IGuiCommon
	stateAccessor             types.IStateAccessor
	layoutManager             *layout.LayoutManager
	inputComponent            types.Component
	ConfirmationComponent     *component.ConfirmationComponent
	eventBus                  events.EventBus
	logger                    types.Logger
	setActiveConfirmationType func(string)
}

func NewToolConfirmationController(
	gui types.IGuiCommon,
	stateAccessor types.IStateAccessor,
	layoutManager *layout.LayoutManager,
	inputComponent types.Component,
	eventBus events.EventBus,
	logger types.Logger,
	setActiveConfirmationType func(string),
) *ToolConfirmationController {
	controller := ToolConfirmationController{
		ConfirmationKeyHandler:    NewConfirmationKeyHandler(),
		gui:                       gui,
		stateAccessor:             stateAccessor,
		layoutManager:             layoutManager,
		inputComponent:            inputComponent,
		eventBus:                  eventBus,
		logger:                    logger,
		setActiveConfirmationType: setActiveConfirmationType,
	}

	eventBus.Subscribe("tool.confirmation.request", func(e interface{}) {
		if event, ok := e.(core_events.ToolConfirmationRequest); ok {
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			controller.HandleToolConfirmationRequest(event)

		}
	})

	return &controller
}

func (tc *ToolConfirmationController) HandleToolConfirmationRequest(event events.ToolConfirmationRequest) error {
	// Set confirmation state
	tc.stateAccessor.SetWaitingConfirmation(true)

	// Always create a new confirmation component for tool confirmations
	tc.ConfirmationComponent = component.NewConfirmationComponent(
		tc.gui,
		event.ExecutionID,
		"1 - Yes | 2 - No",
		nil, // No callback needed since keybindings are handled globally
	)

	// Set the active confirmation type so global keybindings route correctly
	tc.setActiveConfirmationType("tool")

	// Swap to confirmation component
	tc.layoutManager.SwapComponent("input", tc.ConfirmationComponent)

	// Apply secondary theme color to border and title after swap
	tc.gui.GetGui().Update(func(g *gocui.Gui) error {
		if view, err := g.View("input"); err == nil {
			// Get secondary color from theme
			theme := tc.gui.GetTheme()
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
	tc.eventBus.Publish("tool.confirmation.response", events.ToolConfirmationResponse{
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

	// Update UI state to track the focused panel
	tc.stateAccessor.SetFocusedPanel(panelName)
	return nil
}
