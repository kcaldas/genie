package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	core_events "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

// finishedRequestMemory bounds how many completed request IDs are
// remembered so that late-arriving chunks (delivered on a different
// bus topic than the response) cannot resurrect a finished message.
const finishedRequestMemory = 16

type ChatController struct {
	*BaseController
	genie           genie.Genie
	stateAccessor   types.IStateAccessor
	commandEventBus *events.CommandEventBus

	// Request context management
	requestManager *helpers.RequestContextManager
	todoFormatter  *presentation.TodoFormatter

	// Streaming state. chat.chunk and chat.response are distinct bus
	// topics and therefore delivered on distinct goroutines, so all
	// access must hold streamingMu.
	streamingMu      sync.Mutex
	streamingMsgs    map[string]*streamingMessage
	finishedRequests []string
}

type streamingMessage struct {
	messageID int64
	builder   strings.Builder
}

func NewChatController(
	ctx types.Component,
	gui types.Gui,
	genieService genie.Genie,
	state types.IStateAccessor,
	configManager *helpers.ConfigManager,
	commandEventBus *events.CommandEventBus,
) *ChatController {
	c := &ChatController{
		BaseController:  NewBaseController(ctx, gui, configManager),
		genie:           genieService,
		stateAccessor:   state,
		commandEventBus: commandEventBus,
		requestManager:  helpers.NewRequestContextManager(commandEventBus),
		streamingMsgs:   make(map[string]*streamingMessage),
	}

	c.todoFormatter = presentation.NewTodoFormatter(c.GetTheme())

	eventBus := genieService.GetEventBus()
	eventBus.Subscribe("chat.response", func(e interface{}) {
		if event, ok := e.(core_events.ChatResponseEvent); ok {
			c.logger().Debug("Event consumed", "topic", event.Topic())

			// Finish the request
			c.requestManager.FinishRequest()

			canceled := errors.Is(event.Error, context.Canceled)

			if buffer, ok := c.takeStreamingMessage(event.RequestID); ok {
				if event.Error != nil {
					if !canceled {
						c.stateAccessor.UpdateMessageByID(buffer.messageID, func(msg *types.Message) {
							msg.Role = "error"
							msg.Content = fmt.Sprintf("Error: %v", event.Error)
							msg.ContentType = "text"
						})
					}
				} else {
					content := event.Response
					if strings.TrimSpace(content) == "" {
						content = buffer.builder.String()
					}
					c.stateAccessor.UpdateMessageByID(buffer.messageID, func(msg *types.Message) {
						msg.Role = "assistant"
						msg.Content = content
						msg.ContentType = "markdown"
					})
				}

				if !canceled {
					c.renderMessages()
				}
				return
			}

			if event.Error != nil {
				// Don't show context cancellation errors as they're user-initiated
				if !canceled {
					state.AddMessage(types.Message{
						Role:    "error",
						Content: fmt.Sprintf("Error: %v", event.Error),
					})
					c.logger().Debug("Chat failed", "error", event.Error)
				} else {
					c.logger().Debug("Chat canceled by user")
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
			c.logger().Debug("Event consumed", "topic", event.Topic())
			state.AddMessage(types.Message{
				Role:    "system",
				Content: event.Message,
			})
			c.renderMessages()
		}
	})

	eventBus.Subscribe("chat.notification", func(e interface{}) {
		if event, ok := e.(core_events.NotificationEvent); ok {
			c.logger().Debug("Event consumed", "topic", event.Topic())
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

	eventBus.Subscribe("chat.chunk", func(e interface{}) {
		if event, ok := e.(core_events.ChatChunkEvent); ok {
			c.logger().Debug("Event consumed", "topic", event.Topic())
			c.handleChatChunk(event)
		}
	})

	eventBus.Subscribe("tool.executed", func(e interface{}) {
		if event, ok := e.(core_events.ToolExecutedEvent); ok {
			c.logger().Debug("Event consumed", "topic", event.Topic())
			// Check if tool execution should be hidden
			config := c.GetConfig()
			if toolConfig, exists := config.ToolConfigs[event.ToolName]; exists && toolConfig.Hide {
				return // Skip showing this tool execution
			}

			// Format the function call display for chat
			formattedCall := presentation.FormatToolCall(event.ToolName, event.Parameters, c.GetConfig())

			// Add formatted call to chat messages
			// Use assistant role for success (green) and error role for failures (red)
			role := "assistant"
			if !event.Success {
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

	// NEW: Subscribe to tool.confirmation.response
	eventBus.Subscribe("tool.confirmation.response", func(e interface{}) {
		if event, ok := e.(core_events.ToolConfirmationResponse); ok {
			c.logger().Debug("Event consumed", "topic", event.Topic(), "confirmed", event.Confirmed)
			if !event.Confirmed {
				c.CancelChat()
			}
		}
	})

	// Subscribe to user confirmation requests (rich confirmations with content preview)
	eventBus.Subscribe("user.confirmation.request", func(e interface{}) {
		if event, ok := e.(core_events.UserConfirmationRequest); ok {
			c.logger().Debug("Event consumed", "topic", event.Topic())
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

	// NEW: Subscribe to user.confirmation.response
	eventBus.Subscribe("user.confirmation.response", func(e interface{}) {
		if event, ok := e.(core_events.UserConfirmationResponse); ok {
			c.logger().Debug("Event consumed", "topic", event.Topic(), "confirmed", event.Confirmed)
			if !event.Confirmed {
				c.CancelChat()
			}
		}
	})

	// Subscribe to token count events
	eventBus.Subscribe("token.count", func(e interface{}) {
		if event, ok := e.(core_events.TokenCountEvent); ok {
			c.logger().Debug("Event consumed", "topic", event.Topic())
			commandEventBus.Emit("token.count", event.TotalTokens)
		}
	})

	// Subscribe to user input events (only text now - commands handled by CommandHandler)
	commandEventBus.Subscribe("user.input.text", func(event interface{}) {
		if message, ok := event.(string); ok {
			c.logger().Debug("Processing user input", "message", message)
			c.handleChatMessage(message)
			c.renderMessages()
		}
	})

	// Subscribe to user cancel input
	commandEventBus.Subscribe("user.input.cancel", func(event interface{}) {
		c.CancelChat()
		c.renderMessages()
	})

	// Subscribe to persona swap event
	commandEventBus.Subscribe("persona.changed", func(event interface{}) {
		c.logger().Debug("Event consumed", "topic", "persona.changed")
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

// logger returns the current global logger (updated dynamically when debug is toggled)
func (c *ChatController) logger() logging.Logger {
	return logging.GetGlobalLogger()
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
	if err := c.genie.Chat(ctx, message, genie.WithStreaming(true)); err != nil {
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

func (c *ChatController) handleChatChunk(event core_events.ChatChunkEvent) {
	if event.Chunk == nil {
		return
	}
	if text := event.Chunk.Text; text != "" {
		c.appendStreamingText(event.RequestID, text)
	}
}

func (c *ChatController) appendStreamingText(requestID, text string) {
	if text == "" {
		return
	}

	c.streamingMu.Lock()
	defer c.streamingMu.Unlock()

	// A chunk may arrive after its request's response has been handled
	// (chunks and responses travel on different topics). Ignore it
	// instead of resurrecting a finished message.
	for _, finished := range c.finishedRequests {
		if finished == requestID {
			return
		}
	}

	buffer, exists := c.streamingMsgs[requestID]
	canUpdateExisting := false

	if exists {
		if lastMsg := c.stateAccessor.GetLastMessage(); lastMsg != nil &&
			lastMsg.ID == buffer.messageID && lastMsg.Role == "assistant" {
			canUpdateExisting = true
		}
	}

	if !canUpdateExisting {
		msg := types.Message{
			Role:        "assistant",
			Content:     text,
			ContentType: "markdown",
		}
		id := c.stateAccessor.AddMessage(msg)
		buffer = &streamingMessage{messageID: id}
		buffer.builder.WriteString(text)
		c.streamingMsgs[requestID] = buffer
	} else {
		buffer.builder.WriteString(text)
		c.stateAccessor.UpdateMessageByID(buffer.messageID, func(msg *types.Message) {
			msg.Content = buffer.builder.String()
		})
	}
	c.renderMessages()
}

// takeStreamingMessage removes and returns the streaming buffer for a
// request, marking the request as finished so late chunks are dropped.
func (c *ChatController) takeStreamingMessage(requestID string) (*streamingMessage, bool) {
	c.streamingMu.Lock()
	defer c.streamingMu.Unlock()

	c.finishedRequests = append(c.finishedRequests, requestID)
	if len(c.finishedRequests) > finishedRequestMemory {
		c.finishedRequests = c.finishedRequests[len(c.finishedRequests)-finishedRequestMemory:]
	}

	buffer, ok := c.streamingMsgs[requestID]
	if ok {
		delete(c.streamingMsgs, requestID)
	}
	return buffer, ok
}

func (c *ChatController) ClearConversation() error {
	c.stateAccessor.ClearMessages()
	c.renderMessages()
	// TODO: Implement session reset when integrated with proper Genie service
	return nil
}

func (c *ChatController) renderMessages() {
	c.gui.PostUIUpdate(func() {
		// TODO: Handle render error
		_ = c.GetComponent().Render()
	})
}

func (c *ChatController) GetConversationHistory() []types.Message {
	return c.stateAccessor.GetMessages()
}

func (c *ChatController) CancelChat() {
	cancelledCount := c.requestManager.CancelAll()

	if cancelledCount > 0 {
		c.logger().Debug("Cancelled active chat requests", "count", cancelledCount)

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

// ShowWelcomeMessage displays a persona-aware welcome message
func (c *ChatController) ShowWelcomeMessage() {
	personaName := "Genie" // default

	// Try to get the current session and persona
	session, err := c.genie.GetSession()
	if err == nil && session != nil {
		persona := session.GetPersona()
		if persona != nil {
			personaName = persona.GetName()
		}
	}

	welcomeMsg := fmt.Sprintf("Hello! I'm %s! Type :? for help.", personaName)
	c.AddSystemMessage(welcomeMsg)

	if missing := c.genie.MissingTools(); len(missing) > 0 {
		c.AddSystemMessage(fmt.Sprintf("⚠ %d tool(s) not available: %v", len(missing), missing))
	}

	// Emit persona change event to update title
	c.commandEventBus.Emit("persona.changed", map[string]interface{}{
		"name": personaName,
	})
}

func (c *ChatController) AddErrorMessage(message string) {
	c.stateAccessor.AddMessage(types.Message{
		Role:    "error",
		Content: message,
	})
	c.renderMessages()
}

func (c *ChatController) AddAssistantMessage(message string) {
	c.stateAccessor.AddMessage(types.Message{
		Role:        "assistant",
		Content:     message,
		ContentType: "markdown",
	})
	c.renderMessages()
}
