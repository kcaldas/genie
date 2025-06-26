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
			g, err := di.ProvideGenie()
			if err != nil {
				return err
			}
			eventBus := g.GetEventBus()
			return runAskCommand(cmd, args, g, eventBus)
		},
	}

	// Add --accept-all flag for automatic confirmation
	cmd.Flags().Bool("accept-all", false, "Automatically accept all confirmations (useful for scripting)")

	return cmd
}

// NewAskCommandWithGenie creates an ask command that uses a pre-initialized Genie instance
func NewAskCommandWithGenie(genieProvider func() (genie.Genie, *genie.Session)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Ask the AI a question",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, session := genieProvider()
			eventBus := g.GetEventBus()
			return runAskCommandWithSession(cmd, args, g, session, eventBus)
		},
	}

	// Add --accept-all flag for automatic confirmation
	cmd.Flags().Bool("accept-all", false, "Automatically accept all confirmations (useful for scripting)")

	return cmd
}

// runAskCommandWithSession runs the ask command using a pre-created session
func runAskCommandWithSession(cmd *cobra.Command, args []string, g genie.Genie, session *genie.Session, eventBus events.EventBus) error {
	message := strings.Join(args, " ")

	// Check if --accept-all flag is set
	acceptAll, _ := cmd.Flags().GetBool("accept-all")

	// Create channel to wait for response
	responseChan := make(chan events.ChatResponseEvent, 1)

	// Subscribe to event bus directly for chat responses
	eventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(events.ChatResponseEvent); ok {
			responseChan <- resp
		}
	})

	// If --accept-all is enabled, automatically respond to confirmation requests
	if acceptAll {
		// Auto-approve regular tool confirmations
		eventBus.Subscribe("tool.confirmation.request", func(event interface{}) {
			if confirmationEvent, ok := event.(events.ToolConfirmationRequest); ok {
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
			if confirmEvent, ok := event.(events.UserConfirmationRequest); ok {
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
	err := g.Chat(context.Background(), message)
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

func runAskCommand(cmd *cobra.Command, args []string, g genie.Genie, eventBus events.EventBus) error {
	message := strings.Join(args, " ")

	// Check if --accept-all flag is set
	acceptAll, _ := cmd.Flags().GetBool("accept-all")

	// Start Genie and get initial session
	_, err := g.Start(nil) // Use current working directory
	if err != nil {
		return fmt.Errorf("failed to start Genie: %w", err)
	}

	// Create channel to wait for response
	responseChan := make(chan events.ChatResponseEvent, 1)

	// Subscribe to event bus directly for chat responses
	eventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(events.ChatResponseEvent); ok {
			responseChan <- resp
		}
	})

	// If --accept-all is enabled, automatically respond to confirmation requests
	if acceptAll {
		// Auto-approve regular tool confirmations
		eventBus.Subscribe("tool.confirmation.request", func(event interface{}) {
			if confirmationEvent, ok := event.(events.ToolConfirmationRequest); ok {
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
			if confirmEvent, ok := event.(events.UserConfirmationRequest); ok {
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
	err = g.Chat(context.Background(), message)
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
