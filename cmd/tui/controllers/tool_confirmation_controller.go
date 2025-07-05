package controllers

import (
	"fmt"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/events"
)

type ToolConfirmationController struct {
	gui                   types.IGuiCommon
	stateAccessor         types.IStateAccessor
	layoutManager         types.ILayoutManager
	inputComponent        types.Component
	statusComponent       types.IStatusComponent
	ConfirmationComponent *component.ConfirmationComponent
	eventBus              events.EventBus
	onRenderMessages      func() error
	onFocusView           func(string) error
	setActiveConfirmationType func(string)
}

func NewToolConfirmationController(
	gui types.IGuiCommon,
	stateAccessor types.IStateAccessor,
	layoutManager types.ILayoutManager,
	inputComponent types.Component,
	statusComponent types.IStatusComponent,
	eventBus events.EventBus,
	onRenderMessages func() error,
	onFocusView func(string) error,
	setActiveConfirmationType func(string),
) *ToolConfirmationController {
	return &ToolConfirmationController{
		gui:                       gui,
		stateAccessor:             stateAccessor,
		layoutManager:             layoutManager,
		inputComponent:            inputComponent,
		statusComponent:           statusComponent,
		eventBus:                  eventBus,
		onRenderMessages:          onRenderMessages,
		onFocusView:               onFocusView,
		setActiveConfirmationType: setActiveConfirmationType,
	}
}

func (tc *ToolConfirmationController) HandleToolConfirmationRequest(event events.ToolConfirmationRequest) error {
	// Show confirmation message in chat
	tc.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: event.Message,
	})

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
	tc.layoutManager.SetWindowComponent("input", tc.ConfirmationComponent)

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

	// Refresh UI to show the message and swapped component
	if err := tc.onRenderMessages(); err != nil {
		return err
	}
	if err := tc.ConfirmationComponent.Render(); err != nil {
		return err
	}

	// Focus the confirmation component (same view name as input)
	return tc.onFocusView("input")
}

func (tc *ToolConfirmationController) HandleToolConfirmationResponse(executionID string, confirmed bool) error {
	// Clear confirmation state
	tc.stateAccessor.SetWaitingConfirmation(false)

	// Reset status back to ready indicator
	tc.statusComponent.SetLeftToReady()

	// Publish confirmation response
	tc.stateAccessor.AddDebugMessage(fmt.Sprintf("Event published: tool.confirmation.response (confirmed=%v)", confirmed))
	tc.eventBus.Publish("tool.confirmation.response", events.ToolConfirmationResponse{
		ExecutionID: executionID,
		Confirmed:   confirmed,
	})

	// Swap back to input component
	tc.layoutManager.SetWindowComponent("input", tc.inputComponent)

	// Re-render to update the view
	tc.gui.GetGui().Update(func(g *gocui.Gui) error {
		if err := tc.inputComponent.Render(); err != nil {
			return err
		}
		// Focus back on input
		return tc.onFocusView("input")
	})

	return nil
}