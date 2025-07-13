package controllers

import (
	"fmt"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	core_events "github.com/kcaldas/genie/pkg/events"
)

type UserConfirmationController struct {
	*ConfirmationKeyHandler
	gui                   types.Gui
	stateAccessor         types.IStateAccessor
	layoutManager         *layout.LayoutManager
	inputComponent        types.Component
	configManager         *helpers.ConfigManager
	ConfirmationComponent *component.ConfirmationComponent
	diffViewerComponent   *component.DiffViewerComponent
	eventBus              core_events.EventBus
	commandEventBus       *events.CommandEventBus
	logger                types.Logger

	// Queue management
	confirmationQueue      []core_events.UserConfirmationRequest
	processingConfirmation bool
	currentContentType     string // Track content type for the current confirmation
}

func NewUserConfirmationController(
	gui types.Gui,
	stateAccessor types.IStateAccessor,
	layoutManager *layout.LayoutManager,
	inputComponent types.Component,
	diffViewerComponent *component.DiffViewerComponent,
	configManager *helpers.ConfigManager,
	eventBus core_events.EventBus,
	commandEventBus *events.CommandEventBus,
	logger types.Logger,
) *UserConfirmationController {
	c := UserConfirmationController{
		ConfirmationKeyHandler: NewConfirmationKeyHandler(),
		gui:                    gui,
		stateAccessor:          stateAccessor,
		layoutManager:          layoutManager,
		inputComponent:         inputComponent,
		diffViewerComponent:    diffViewerComponent,
		configManager:          configManager,
		eventBus:               eventBus,
		commandEventBus:        commandEventBus,
		logger:                 logger,
	}
	eventBus.Subscribe("user.confirmation.request", func(e interface{}) {
		if event, ok := e.(core_events.UserConfirmationRequest); ok {
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			c.HandleUserConfirmationRequest(event)
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

func (uc *UserConfirmationController) HandleUserConfirmationRequest(event core_events.UserConfirmationRequest) error {
	// Set confirmation state
	uc.stateAccessor.SetWaitingConfirmation(true)
	// Add to queue if we're already processing a confirmation
	if uc.processingConfirmation {
		uc.confirmationQueue = append(uc.confirmationQueue, event)

		// Show queued message to user
		queuePosition := len(uc.confirmationQueue)
		uc.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: fmt.Sprintf("Confirmation request queued (position %d): %s", queuePosition, event.Message),
		})
		return nil
	}

	// Mark as processing
	uc.processingConfirmation = true

	return uc.processConfirmationRequest(event)
}

func (uc *UserConfirmationController) processConfirmationRequest(event core_events.UserConfirmationRequest) error {
	title := event.Title
	if title == "" {
		title = "Confirm Action"
	}

	confirmText := event.ConfirmText
	if confirmText == "" {
		confirmText = "Confirm"
	}

	cancelText := event.CancelText
	if cancelText == "" {
		cancelText = "Cancel"
	}

	// Always create a new confirmation component for user confirmations
	uc.ConfirmationComponent = component.NewConfirmationComponent(
		uc.gui,
		uc.configManager,
		event.ExecutionID,
		fmt.Sprintf("1 - %s | 2 - %s", confirmText, cancelText),
		uc.HandleUserConfirmationResponse, // Connect to controller's response handler
	)

	// Store the content type for this confirmation and set active type
	uc.currentContentType = event.ContentType
	uc.stateAccessor.SetActiveConfirmationType("user")

	// Swap to confirmation component
	uc.layoutManager.SetComponent("input", uc.ConfirmationComponent)

	// Apply secondary theme color to border and title after swap
	uc.gui.GetGui().Update(func(g *gocui.Gui) error {
		if view, err := g.View("input"); err == nil {
			// Get secondary color from theme
			theme := uc.configManager.GetTheme()
			if theme != nil {
				// Convert secondary color to gocui color for border and title
				secondaryColor := presentation.ConvertAnsiToGocuiColor(theme.Secondary)
				view.FrameColor = secondaryColor
				view.TitleColor = secondaryColor
			}
		}
		return nil
	})

	uc.gui.PostUIUpdate(func() {
		if err := uc.ConfirmationComponent.Render(); err != nil {
			// TODO handle render error
		}
	})

	// Focus the confirmation component (same view name as input)
	if err := uc.focusPanelByName("input"); err != nil {
		return err
	}

	// Show diff in right panel AFTER confirmation UI is set up
	if event.ContentType == "diff" && event.Content != "" {
		diffTitle := title
		if event.FilePath != "" {
			diffTitle = fmt.Sprintf("Diff: %s", event.FilePath)
		}

		uc.showDiffInViewer(event.Content, diffTitle)
	}

	return nil
}

func (uc *UserConfirmationController) showDiffInViewer(diffContent, title string) {
	// Show the right panel first
	uc.layoutManager.ShowRightPanel("diff-viewer")

	// Set content first
	uc.diffViewerComponent.SetContent(diffContent)
	uc.diffViewerComponent.SetTitle(title)

	time.Sleep(50 * time.Millisecond)

	// Use a separate GUI update for rendering to avoid race conditions
	uc.gui.PostUIUpdate(func() {
		// Ensure the view exists before rendering
		if view, err := uc.gui.GetGui().View("diff-viewer"); err == nil && view != nil {
			uc.diffViewerComponent.Render()
		}
		// If view doesn't exist yet, that's ok - it will render on next cycle
	})
}

// HandleKeyPress processes a key press and determines if it's a confirmation response
func (uc *UserConfirmationController) HandleKeyPress(key interface{}) (bool, error) {
	// Check if we have an active confirmation
	if uc.ConfirmationComponent == nil {
		return false, nil
	}

	// Use the embedded key handler to interpret the key
	confirmed, handled := uc.InterpretKey(key)
	if handled {
		executionID := uc.ConfirmationComponent.ExecutionID
		return true, uc.HandleUserConfirmationResponse(executionID, confirmed)
	}

	return false, nil
}

func (uc *UserConfirmationController) HandleUserConfirmationResponse(executionID string, confirmed bool) error {
	// Clear confirmation state
	uc.stateAccessor.SetWaitingConfirmation(false)

	// Hide diff viewer if it was shown
	if uc.currentContentType == "diff" {
		uc.layoutManager.HideRightPanel()
	}

	// Publish confirmation response
	uc.logger.Debug(fmt.Sprintf("Event published: user.confirmation.response (confirmed=%v)", confirmed))
	uc.eventBus.Publish("user.confirmation.response", core_events.UserConfirmationResponse{
		ExecutionID: executionID,
		Confirmed:   confirmed,
	})

	// Process next confirmation from queue
	return uc.processNextConfirmation()
}

func (uc *UserConfirmationController) processNextConfirmation() error {
	// Check if there are more confirmations in the queue
	if len(uc.confirmationQueue) > 0 {
		// Get the next confirmation from the queue
		nextEvent := uc.confirmationQueue[0]
		uc.confirmationQueue = uc.confirmationQueue[1:] // Remove from queue

		// Process it immediately
		return uc.processConfirmationRequest(nextEvent)
	}

	// No more confirmations - restore input component and mark as not processing
	uc.processingConfirmation = false
	uc.layoutManager.SetComponent("input", uc.inputComponent)

	// Re-render to update the view
	uc.gui.GetGui().Update(func(g *gocui.Gui) error {
		if err := uc.inputComponent.Render(); err != nil {
			return err
		}
		// Focus back on input
		return uc.focusPanelByName("input")
	})

	return nil
}

func (uc *UserConfirmationController) focusPanelByName(panelName string) error {
	// Delegate to layout manager for panel focusing
	if err := uc.layoutManager.FocusPanel(panelName); err != nil {
		return err
	}
	return nil
}

// GetConfirmationQueueStatus returns information about pending confirmations
func (uc *UserConfirmationController) GetConfirmationQueueStatus() (int, bool) {
	return len(uc.confirmationQueue), uc.processingConfirmation
}
