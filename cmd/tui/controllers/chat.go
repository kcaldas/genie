package controllers

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
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
	logger          types.Logger

	// Request context management
	requestManager *helpers.RequestContextManager
	todoFormatter  *presentation.TodoFormatter
}

func NewChatController(
	ctx types.Component,
	gui types.Gui,
	genieService genie.Genie,
	state types.IStateAccessor,
	configManager *helpers.ConfigManager,
	commandEventBus *events.CommandEventBus,
	logger types.Logger,
) *ChatController {
	c := &ChatController{
		BaseController:  NewBaseController(ctx, gui, configManager),
		genie:           genieService,
		stateAccessor:   state,
		commandEventBus: commandEventBus,
		logger:          logger,
		requestManager:  helpers.NewRequestContextManager(commandEventBus),
	}

	c.todoFormatter = presentation.NewTodoFormatter(c.GetTheme())

	eventBus := genieService.GetEventBus()
	eventBus.Subscribe("chat.response", func(e interface{}) {
		if event, ok := e.(core_events.ChatResponseEvent); ok {
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))

			// Finish the request
			c.requestManager.FinishRequest()

			if event.Error != nil {
				// Don't show context cancellation errors as they're user-initiated
				if !strings.Contains(event.Error.Error(), "context canceled") {
					state.AddMessage(types.Message{
						Role:    "error",
						Content: fmt.Sprintf("Error: %v", event.Error),
					})
					logger.Debug(fmt.Sprintf("Chat failed: %v", event.Error))
				} else {
					logger.Debug("Chat canceled by the user.")
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
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			state.AddMessage(types.Message{
				Role:    "system",
				Content: event.Message,
			})
			c.renderMessages()
		}
	})

	eventBus.Subscribe("chat.notification", func(e interface{}) {
		if event, ok := e.(core_events.NotificationEvent); ok {
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			role := "assistant"
			if event.Role != "" {
				role = event.Role
			}
			state.AddMessage(types.Message{
				Role:        role,
				Content:     event.Message,
				ContentType: event.ContentType,
			})
			c.renderMessages()
		}
	})

	eventBus.Subscribe("tool.executed", func(e interface{}) {
		if event, ok := e.(core_events.ToolExecutedEvent); ok {
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			// Check if tool execution should be hidden
			config := c.GetConfig()
			if toolConfig, exists := config.ToolConfigs[event.ToolName]; exists && toolConfig.Hide {
				return // Skip showing this tool execution
			}

			// Format the function call display for chat
			formattedCall := presentation.FormatToolCall(event.ToolName, event.Parameters, c.GetConfig())

			// Determine success based on the message (no "Failed:" prefix means success)
			success := !strings.HasPrefix(event.Message, "Failed:")

			// Add formatted call to chat messages
			// Use assistant role for success (green) and error role for failures (red)
			role := "assistant"
			if !success {
				role = "error"
			}

			// Format the result preview
			resultPreview := presentation.FormatToolResult(event.ToolName, event.Result, c.todoFormatter, c.GetConfig())

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
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
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
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
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

	// Subscribe to token count events
	eventBus.Subscribe("token.count", func(e interface{}) {
		if event, ok := e.(core_events.TokenCountEvent); ok {
			logger.Debug(fmt.Sprintf("Event consumed: %s", event.Topic()))
			commandEventBus.Emit("token.count", event.TotalTokens)
		}
	})

	// Subscribe to user input events (only text now - commands handled by CommandHandler)
	commandEventBus.Subscribe("user.input.text", func(event interface{}) {
		if message, ok := event.(string); ok {
			c.handleChatMessage(message)
			c.renderMessages()
		}
	})

	// Subscribe to user cancel input
	commandEventBus.Subscribe("user.input.cancel", func(event interface{}) {
		c.CancelChat()
		c.renderMessages()
	})

	// Subscribe to theme changes for app-level updates
	commandEventBus.Subscribe("theme.changed", func(event interface{}) {
		if eventData, ok := event.(map[string]interface{}); ok {
			if config, ok := eventData["config"].(*types.Config); ok {
				// Update todoFormatter with new theme
				theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
				c.todoFormatter = presentation.NewTodoFormatter(theme)
			}
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

	// Start a new request and get the shared context
	ctx := c.requestManager.StartRequest()

	// Use the shared context for this request
	if err := c.genie.Chat(ctx, message); err != nil {
		// Clean up on immediate failure
		c.requestManager.FinishRequest()

		c.stateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Failed to send message: %v", err),
		})
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
	cancelledCount := c.requestManager.CancelAll()

	if cancelledCount > 0 {
		c.logger.Debug(fmt.Sprintf("Cancelled %d active chat requests", cancelledCount))

		c.stateAccessor.AddMessage(types.Message{
			Role:    "system",
			Content: "All chat requests cancelled",
		})
	}
}

func (c *ChatController) AddSystemMessage(message string) {
	c.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: message,
	})
	c.renderMessages()
}

func (c *ChatController) AddErrorMessage(message string) {
	c.stateAccessor.AddMessage(types.Message{
		Role:    "error",
		Content: message,
	})
	c.renderMessages()
}
