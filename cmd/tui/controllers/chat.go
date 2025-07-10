package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	core_events "github.com/kcaldas/genie/pkg/events"
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
	c := &ChatController{
		BaseController:  NewBaseController(ctx, gui),
		genie:           genieService,
		stateAccessor:   state,
		commandEventBus: commandEventBus,
	}

	todoFormatter := presentation.NewTodoFormatter(gui.GetTheme())

	eventBus := genieService.GetEventBus()
	eventBus.Subscribe("chat.response", func(e interface{}) {
		if event, ok := e.(core_events.ChatResponseEvent); ok {
			state.SetLoading(false)
			if event.Error != nil {
				// Don't show context cancellation errors as they're user-initiated
				if !strings.Contains(event.Error.Error(), "context canceled") {
					state.AddMessage(types.Message{
						Role:    "error",
						Content: fmt.Sprintf("Error: %v", event.Error),
					})
				}
			} else {
				state.AddMessage(types.Message{
					Role:        "assistant",
					Content:     event.Response,
					ContentType: "markdown",
				})
			}
			c.renderMessages()
		}
	})

	eventBus.Subscribe("tool.call.message", func(e interface{}) {
		if event, ok := e.(core_events.ToolCallMessageEvent); ok {
			state.AddMessage(types.Message{
				Role:    "system",
				Content: event.Message,
			})
			c.renderMessages()
		}
	})

	eventBus.Subscribe("tool.executed", func(e interface{}) {
		if event, ok := e.(core_events.ToolExecutedEvent); ok {
			// Skip TodoRead - don't show it in chat at all
			if event.ToolName == "TodoRead" {
				return
			}

			// Format the function call display for chat
			formattedCall := presentation.FormatToolCall(event.ToolName, event.Parameters, gui.GetConfig())

			// Determine success based on the message (no "Failed:" prefix means success)
			success := !strings.HasPrefix(event.Message, "Failed:")

			// Add formatted call to chat messages
			// Use assistant role for success (green) and error role for failures (red)
			role := "assistant"
			if !success {
				role = "error"
			}

			// Format the result preview
			resultPreview := presentation.FormatToolResult(event.ToolName, event.Result, todoFormatter, gui.GetConfig())

			chatMsg := formattedCall + resultPreview
			state.AddMessage(types.Message{
				Role:    role,
				Content: chatMsg,
			})

			c.renderMessages()
		}
	})

	eventBus.Subscribe("tool.confirmation.request", func(e interface{}) {
		if event, ok := e.(core_events.ToolConfirmationRequest); ok {
			// Show confirmation message in chat
			state.AddMessage(types.Message{
				Role:    "system",
				Content: event.Message,
			})

			c.renderMessages()
		}
	})

	// Subscribe to user confirmation requests (rich confirmations with content preview)
	eventBus.Subscribe("user.confirmation.request", func(e interface{}) {
		if event, ok := e.(core_events.UserConfirmationRequest); ok {
			message := event.Message
			if message == "" {
				if event.FilePath != "" {
					message = fmt.Sprintf("Do you want to proceed with changes to %s?", event.FilePath)
				} else {
					message = "Do you want to proceed?"
				}
			}

			// Show confirmation message in chat
			state.AddMessage(types.Message{
				Role:    "system",
				Content: message,
			})
			c.renderMessages()
		}
	})

	// Subscribe to user input events (only text now - commands handled by CommandHandler)
	commandEventBus.Subscribe("user.input.text", func(event interface{}) {
		if message, ok := event.(string); ok {
			c.handleChatMessage(message)
			c.renderMessages()
		}
	})

	return c
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
	c.renderMessages()
	// TODO: Implement session reset when integrated with proper Genie service
	return nil
}

func (c *ChatController) renderMessages() {
	c.gui.PostUIUpdate(func() {
		if err := c.GetComponent().Render(); err != nil {
			// TODO: Handle render error
		}
	})
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

func (c *ChatController) ShowWelcomeMessage() {
	c.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: "Welcome to Genie! Type :? for help.",
	})
	c.renderMessages()
}
