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

type UserConfirmationController struct {
	*ConfirmationKeyHandler
	gui                   types.Gui
	stateAccessor         types.IStateAccessor
	layoutManager         *layout.LayoutManager
	inputComponent        types.Component
	configManager         *helpers.ConfigManager
	ConfirmationComponent *component.ConfirmationComponent
	diffViewerComponent   *component.DiffViewerComponent
	textViewerComponent   *component.TextViewerComponent
	eventBus              core_events.EventBus
	commandEventBus       *events.CommandEventBus

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
	textViewerComponent *component.TextViewerComponent,
	configManager *helpers.ConfigManager,
	eventBus core_events.EventBus,
	commandEventBus *events.CommandEventBus,
) *UserConfirmationController {
	c := UserConfirmationController{
		ConfirmationKeyHandler: NewConfirmationKeyHandler(),
		gui:                    gui,
		stateAccessor:          stateAccessor,
		layoutManager:          layoutManager,
		inputComponent:         inputComponent,
		diffViewerComponent:    diffViewerComponent,
		textViewerComponent:    textViewerComponent,
		configManager:          configManager,
		eventBus:               eventBus,
		commandEventBus:        commandEventBus,
	}
	eventBus.Subscribe("user.confirmation.request", func(e interface{}) {
		if event, ok := e.(core_events.UserConfirmationRequest); ok {
			logging.GetGlobalLogger().Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			c.HandleUserConfirmationRequest(event)
		}
	})
	// Subscribe to user cancel input
	commandEventBus.Subscribe("user.input.cancel", func(event interface{}) {
		// Skip if no confirmation is being processed
		if !c.processingConfirmation {
			return
		}
		c.stateAccessor.SetWaitingConfirmation(false)
		c.processingConfirmation = false
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
func (uc *UserConfirmationController) logger() logging.Logger {
	return logging.GetGlobalLogger()
}

func (uc *UserConfirmationController) HandleUserConfirmationRequest(event core_events.UserConfirmationRequest) error {
	// Check if tool has auto-accept enabled
	config := uc.configManager.GetConfig()
	if toolConfig, exists := config.ToolConfigs[event.Title]; exists && toolConfig.AutoAccept {
		// Auto-accept without showing dialog
		uc.logger().Debug(fmt.Sprintf("Auto-accepting confirmation for tool: %s", event.Title))
		uc.eventBus.Publish("user.confirmation.response", core_events.UserConfirmationResponse{
			ExecutionID: event.ExecutionID,
			Confirmed:   true,
		})
		return nil
	}

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

	// Determine viewer panel and content
	viewerMode := ""
	var viewerTitle string
	if event.ContentType == "diff" && event.Content != "" {
		viewerMode = "diff-viewer"
		viewerTitle = title
		if event.FilePath != "" {
			viewerTitle = fmt.Sprintf("Diff: %s", event.FilePath)
		}
	} else if event.ContentType == "markdown" && event.Content != "" {
		viewerMode = "text-viewer"
		viewerTitle = title
		if event.FilePath != "" {
			viewerTitle = fmt.Sprintf("Markdown: %s", event.FilePath)
		}
	}
	viewerContent := event.Content

	// All gocui state modifications must run on the main loop
	uc.gui.GetGui().Update(func(g *gocui.Gui) error {
		// Swap to confirmation component
		uc.layoutManager.SwapComponent("input", uc.ConfirmationComponent)

		// Apply secondary theme color to border and title
		if view, err := g.View("input"); err == nil {
			theme := uc.configManager.GetTheme()
			if theme != nil {
				secondaryColor := presentation.ConvertAnsiToGocuiColor(theme.Secondary)
				view.FrameColor = secondaryColor
				view.TitleColor = secondaryColor
			}
		}

		if err := uc.ConfirmationComponent.Render(); err != nil {
			uc.logger().Debug("Failed to render confirmation component", "error", err)
		}

		// Focus the confirmation component
		uc.focusPanelByName("input")

		// Show content in right panel
		if viewerMode == "diff-viewer" {
			uc.layoutManager.ShowRightPanel("diff-viewer")
			uc.diffViewerComponent.SetContent(viewerContent)
			uc.diffViewerComponent.SetTitle(viewerTitle)
		} else if viewerMode == "text-viewer" {
			uc.layoutManager.ShowRightPanel("text-viewer")
			uc.textViewerComponent.SetContentWithType(viewerContent, "markdown")
			uc.textViewerComponent.SetTitle(viewerTitle)
		}

		return nil
	})

	// Queue a follow-up render for the viewer (needs view to exist after Layout)
	if viewerMode != "" {
		uc.gui.PostUIUpdate(func() {
			if viewerMode == "diff-viewer" {
				if view, err := uc.gui.GetGui().View("diff-viewer"); err == nil && view != nil {
					uc.diffViewerComponent.Render()
				}
			} else if viewerMode == "text-viewer" {
				if view, err := uc.gui.GetGui().View("text-viewer"); err == nil && view != nil {
					uc.textViewerComponent.Render()
				}
			}
		})
	}

	return nil
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
	uc.ConfirmationComponent = nil

	// Hide viewer panel if it was shown
	if uc.currentContentType == "diff" || uc.currentContentType == "markdown" {
		uc.layoutManager.HideRightPanel()
	}

	// Publish confirmation response
	uc.logger().Debug(fmt.Sprintf("Event published: user.confirmation.response (confirmed=%v)", confirmed))
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

	// All gocui state modifications must run on the main loop
	uc.gui.GetGui().Update(func(g *gocui.Gui) error {
		uc.layoutManager.SwapComponent("input", uc.inputComponent)
		if err := uc.inputComponent.Render(); err != nil {
			return err
		}
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
