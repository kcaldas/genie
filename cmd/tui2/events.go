package tui2

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/pkg/events"
)

// setupEventSubscriptions sets up event bus subscriptions
func (t *TUI) setupEventSubscriptions() {
	// Subscribe to chat responses
	t.subscriber.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(events.ChatResponseEvent); ok {
			t.g.Update(func(g *gocui.Gui) error {
				t.loading = false
				if resp.Error != nil {
					t.addMessage(ErrorMessage, fmt.Sprintf("Error: %v", resp.Error))
					t.addDebugMessage(fmt.Sprintf("Chat error received: %v", resp.Error))
				} else {
					t.addMessage(AssistantMessage, resp.Response)
					t.addDebugMessage(fmt.Sprintf("Chat response received (%d chars)", len(resp.Response)))
				}
				return nil
			})
		}
	})
	
	// Subscribe to tool executions
	t.subscriber.Subscribe("tool.executed", func(event interface{}) {
		if toolEvent, ok := event.(events.ToolExecutedEvent); ok {
			t.g.Update(func(g *gocui.Gui) error {
				// Format the function call display like Bubble Tea TUI
				formattedCall := formatFunctionCall(toolEvent.ToolName, toolEvent.Parameters)
				
				// Determine success based on message (no "Failed:" prefix means success)
				success := !strings.HasPrefix(toolEvent.Message, "Failed:")
				
				// Create display message with function call and colored circle indicator
				msg := fmt.Sprintf("● %s", formattedCall)
				
				t.addToolMessage(msg, &success)
				t.addDebugMessage(fmt.Sprintf("Tool executed: %s with params %v - %s", 
					toolEvent.ToolName, toolEvent.Parameters, toolEvent.Message))
				return nil
			})
		}
	})
	
	// Subscribe to tool call messages
	t.subscriber.Subscribe("tool.call.message", func(event interface{}) {
		if messageEvent, ok := event.(events.ToolCallMessageEvent); ok {
			t.g.Update(func(g *gocui.Gui) error {
				msg := fmt.Sprintf("● %s", messageEvent.Message)
				t.addMessage(ToolMessage, msg)
				t.addDebugMessage(fmt.Sprintf("Tool call message: %s - %s", messageEvent.ToolName, messageEvent.Message))
				return nil
			})
		}
	})
	
	// Subscribe to tool confirmations
	t.subscriber.Subscribe("tool.confirmation.request", func(event interface{}) {
		if confirmEvent, ok := event.(events.ToolConfirmationRequest); ok {
			t.g.Update(func(g *gocui.Gui) error {
				t.showConfirmationDialog(
					confirmEvent.ToolName,
					confirmEvent.Command,
					func(confirmed bool) {
						response := events.ToolConfirmationResponse{
							ExecutionID: confirmEvent.ExecutionID,
							Confirmed:   confirmed,
						}
						t.publisher.Publish(response.Topic(), response)
						t.addDebugMessage(fmt.Sprintf("Tool confirmation: %s -> %t", confirmEvent.ToolName, confirmed))
					},
				)
				return nil
			})
		}
	})
	
	// Subscribe to user confirmations
	t.subscriber.Subscribe("user.confirmation.request", func(event interface{}) {
		if confirmEvent, ok := event.(events.UserConfirmationRequest); ok {
			t.g.Update(func(g *gocui.Gui) error {
				t.showConfirmationDialog(
					"User Confirmation",
					confirmEvent.Message,
					func(confirmed bool) {
						response := events.UserConfirmationResponse{
							ExecutionID: confirmEvent.ExecutionID,
							Confirmed:   confirmed,
						}
						t.publisher.Publish(response.Topic(), response)
						t.addDebugMessage(fmt.Sprintf("User confirmation: %s -> %t", confirmEvent.Message, confirmed))
					},
				)
				return nil
			})
		}
	})
}