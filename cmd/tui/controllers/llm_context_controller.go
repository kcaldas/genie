package controllers

import (
	"context"
	"fmt"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type LLMContextController struct {
	*BaseController
	genie            genie.Genie
	stateAccessor    types.IStateAccessor
	layoutManager    *layout.LayoutManager
	contextComponent *component.LLMContextViewerComponent
	commandEventBus  *events.CommandEventBus
	logger           types.Logger
	contextData      map[string]string // Store context data in controller
}

func NewLLMContextController(
	gui types.Gui,
	genieService genie.Genie,
	layoutManager *layout.LayoutManager,
	state types.IStateAccessor,
	configManager *helpers.ConfigManager,
	commandEventBus *events.CommandEventBus,
	logger types.Logger,
) *LLMContextController {
	c := &LLMContextController{
		genie:           genieService,
		layoutManager:   layoutManager,
		stateAccessor:   state,
		commandEventBus: commandEventBus,
		logger:          logger,
		contextData:     make(map[string]string),
	}

	// Create the component with controller as the data source
	c.contextComponent = component.NewLLMContextViewerComponent(gui, configManager, c, c.onClose)
	c.BaseController = NewBaseController(c.contextComponent, gui, configManager)

	return c
}

func (c *LLMContextController) onClose() error {
	c.stateAccessor.SetContextViewerActive(false)
	return c.layoutManager.FocusPanel("input")
}

// Show displays the context viewer
func (c *LLMContextController) Show() error {
	// Toggle behavior - close if already open
	if c.stateAccessor.IsContextViewerActive() {
		c.stateAccessor.SetContextViewerActive(false)
		return c.Close()
	}

	// Load context data first
	if err := c.loadContextData(); err != nil {
		return fmt.Errorf("failed to load context data: %w", err)
	}

	// Show the component
	if err := c.contextComponent.Show(); err != nil {
		return err
	}

	// Set up keybindings for the component
	gui := c.gui.GetGui()
	for _, kb := range c.contextComponent.GetKeybindings() {
		if err := gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
			return err
		}
	}

	// Render initial content
	if err := c.contextComponent.Render(); err != nil {
		return err
	}

	// Focus the context keys panel for navigation
	contextKeysViewName := c.contextComponent.GetViewName() + "-context-keys"
	gui.Update(func(g *gocui.Gui) error {
		if v, err := g.View(contextKeysViewName); err == nil {
			_, err := g.SetCurrentView(v.Name())
			return err
		}
		return nil
	})

	c.stateAccessor.SetContextViewerActive(true)

	return nil
}

// Close hides the context viewer
func (c *LLMContextController) Close() error {
	return c.contextComponent.Close()
}

// RefreshContext reloads the context data from Genie
func (c *LLMContextController) RefreshContext() error {
	if err := c.loadContextData(); err != nil {
		c.logger.Debug(fmt.Sprintf("Failed to refresh context: %v", err))
		return err
	}

	c.logger.Debug("Context refreshed successfully")

	// Trigger component re-render
	return c.contextComponent.Render()
}

// GetContextData returns the current context data (called by component)
func (c *LLMContextController) GetContextData() map[string]string {
	return c.contextData
}

// loadContextData fetches context from Genie service
func (c *LLMContextController) loadContextData() error {
	ctx := context.Background()
	contextParts, err := c.genie.GetContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get context: %w", err)
	}

	c.contextData = contextParts
	return nil
}

// HandleComponentEvent processes events from the component
func (c *LLMContextController) HandleComponentEvent(eventName string, data interface{}) error {
	switch eventName {
	case "refresh":
		return c.RefreshContext()
	case "close":
		return c.Close()
	default:
		return fmt.Errorf("unknown event: %s", eventName)
	}
}
