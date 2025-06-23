package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/spf13/cobra"
)

func NewAskCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Ask the AI a question",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := di.InitializeGenie()
			if err != nil {
				return err
			}
			eventBus := di.ProvideEventBus()
			return runAskCommand(cmd, args, g, eventBus)
		},
	}
	
	// Add --accept-all flag for automatic confirmation
	cmd.Flags().Bool("accept-all", false, "Automatically accept all confirmations (useful for scripting)")
	
	return cmd
}

func NewAskCommandWithGenie(g genie.Genie, eventBus events.EventBus) *cobra.Command {
	return &cobra.Command{
		Use:   "ask",
		Short: "Ask the AI a question",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAskCommand(cmd, args, g, eventBus)
		},
	}
}

func runAskCommand(cmd *cobra.Command, args []string, g genie.Genie, eventBus events.EventBus) error {
	message := strings.Join(args, " ")
	
	// Check if --accept-all flag is set
	acceptAll, _ := cmd.Flags().GetBool("accept-all")
	
	// Create session through Genie API
	sessionID, err := g.CreateSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	
	// Create channel to wait for response
	responseChan := make(chan genie.ChatResponseEvent, 1)
	
	// Subscribe to event bus directly for chat responses
	eventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(genie.ChatResponseEvent); ok && resp.SessionID == sessionID {
			responseChan <- resp
		}
	})
	
	// If --accept-all is enabled, automatically respond to confirmation requests
	if acceptAll {
		// Auto-approve regular tool confirmations
		eventBus.Subscribe("tool.confirmation.request", func(event interface{}) {
			if confirmationEvent, ok := event.(events.ToolConfirmationRequest); ok && confirmationEvent.SessionID == sessionID {
				cmd.Printf("Auto-accepting: %s - %s\n", confirmationEvent.ToolName, confirmationEvent.Command)
				response := events.ToolConfirmationResponse{
					ExecutionID: confirmationEvent.ExecutionID,
					Confirmed:   true,
				}
				eventBus.Publish(response.Topic(), response)
			}
		})
		
		// Auto-approve diff confirmations
		eventBus.Subscribe("user.confirmation.request", func(event interface{}) {
			if confirmEvent, ok := event.(events.UserConfirmationRequest); ok && confirmEvent.SessionID == sessionID {
				cmd.Printf("Auto-accepting %s: %s\n", confirmEvent.ContentType, confirmEvent.FilePath)
				response := events.UserConfirmationResponse{
					ExecutionID: confirmEvent.ExecutionID,
					Confirmed:   true,
				}
				eventBus.Publish(response.Topic(), response)
			}
		})
	}
	
	// Start chat with Genie
	err = g.Chat(context.Background(), sessionID, message)
	if err != nil {
		return fmt.Errorf("failed to start chat: %w", err)
	}
	
	// Wait for response (longer timeout for tool operations)
	timeout := 60 * time.Second
	if acceptAll {
		timeout = 120 * time.Second // Even longer for automated operations
	}
	
	select {
	case resp := <-responseChan:
		if resp.Error != nil {
			return fmt.Errorf("chat failed: %w", resp.Error)
		}
		cmd.Println(resp.Response)
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for response")
	}
}
