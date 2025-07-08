package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type ChatController struct {
	*BaseController
	genie           genie.Genie
	stateAccessor   types.IStateAccessor
	commandEventBus *events.CommandEventBus
	
	// Store active context for cancellation
	activeCancel context.CancelFunc
}

func NewChatController(
	ctx types.Component,
	gui types.IGuiCommon,
	genieService genie.Genie,
	state types.IStateAccessor,
	commandEventBus *events.CommandEventBus,
) *ChatController {
	controller := &ChatController{
		BaseController:  NewBaseController(ctx, gui),
		genie:           genieService,
		stateAccessor:   state,
		commandEventBus: commandEventBus,
	}

	// Subscribe to user input events (only text now - commands handled by CommandHandler)
	commandEventBus.Subscribe("user.input.text", func(event interface{}) {
		if message, ok := event.(string); ok {
			controller.handleChatMessage(message)
		}
	})

	return controller
}

func (c *ChatController) HandleInput(input string) error {
	userInput := types.UserInput{
		Message:        input,
		IsCommand: strings.HasPrefix(input, ":"),
	}
	
	if userInput.IsCommand {
		// Commands are now handled directly by CommandHandler via event bus
		return nil
	}
	
	return c.handleChatMessage(userInput.Message)
}

func (c *ChatController) handleChatMessage(message string) error {
	// Add user message to display
	c.stateAccessor.AddMessage(types.Message{
		Role:    "user",
		Content: message,
	})
	
	// Set loading state
	c.stateAccessor.SetLoading(true)
	
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	c.activeCancel = cancel
	
	// Send message to Genie service
	if err := c.genie.Chat(ctx, message); err != nil {
		c.stateAccessor.SetLoading(false)
		c.stateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Failed to send message: %v", err),
		})
		c.activeCancel = nil
		return err
	}
	
	return nil
}

func (c *ChatController) ClearConversation() error {
	c.stateAccessor.ClearMessages()
	// TODO: Implement session reset when integrated with proper Genie service
	return nil
}

func (c *ChatController) GetConversationHistory() []types.Message {
	return c.stateAccessor.GetMessages()
}

func (c *ChatController) CancelChat() {
	if c.activeCancel != nil {
		c.stateAccessor.AddDebugMessage("Chat cancelled by user")
		c.activeCancel()
		c.activeCancel = nil
		c.stateAccessor.SetLoading(false)
		c.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: "Chat cancelled",
		})
	}
}