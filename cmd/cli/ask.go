package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/spf13/cobra"
)


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

	// Add flags
	cmd.Flags().Bool("accept-all", false, "Automatically accept all confirmations (useful for scripting)")
	cmd.Flags().Bool("debug", false, "Enable debug logging for ask command events")

	return cmd
}


// runAskCommandWithSession runs the ask command using a pre-created session
func runAskCommandWithSession(cmd *cobra.Command, args []string, g genie.Genie, session *genie.Session, eventBus events.EventBus) error {
	message := strings.Join(args, " ")

	// Check flags
	acceptAll, _ := cmd.Flags().GetBool("accept-all")
	debug, _ := cmd.Flags().GetBool("debug")
	
	// Check if verbose flag is set from parent command
	verbose := false
	if cmd.Parent() != nil {
		verbose, _ = cmd.Parent().PersistentFlags().GetBool("verbose")
	}

	// Calculate logging level based on flags
	var logLevel slog.Level
	if debug {
		logLevel = slog.LevelDebug
	} else if verbose {
		logLevel = slog.LevelInfo
	} else {
		logLevel = slog.LevelWarn // Only warnings and errors for clean output
	}

	// Temporarily adjust logging level
	originalLogger := logging.GetGlobalLogger()
	askLogger := logging.NewLogger(logging.Config{
		Level:   logLevel,
		Format:  logging.FormatText,
		Output:  cmd.ErrOrStderr(), // Debug/info to stderr, response to stdout
		AddTime: false,
	})
	logging.SetGlobalLogger(askLogger)
	
	// Restore original logger when done
	defer logging.SetGlobalLogger(originalLogger)

	logger := logging.GetGlobalLogger()
	logger.Debug("starting ask command", "accept-all", acceptAll, "debug", debug, "verbose", verbose, "message", message)

	// Create channel to signal completion
	done := make(chan error, 1)

	// Subscribe to event bus directly for chat responses
	logger.Debug("subscribing to chat.response events")
	eventBus.Subscribe("chat.response", func(event interface{}) {
		logger.Debug("received chat.response event")
		if resp, ok := event.(events.ChatResponseEvent); ok {
			logger.Debug("event is valid ChatResponseEvent")
			if resp.Error != nil {
				logger.Debug("chat response has error", "error", resp.Error)
				done <- fmt.Errorf("chat failed: %w", resp.Error)
			} else {
				logger.Debug("chat response successful, printing response")
				cmd.Println(resp.Response)
				done <- nil // Success
			}
		} else {
			logger.Debug("event is not ChatResponseEvent", "event_type", fmt.Sprintf("%T", event))
		}
	})

	// If --accept-all is enabled, automatically respond to confirmation requests
	if acceptAll {
		logger.Debug("setting up auto-confirmation handlers")
		
		// Auto-approve regular tool confirmations
		logger.Debug("subscribing to tool.confirmation.request events")
		eventBus.Subscribe("tool.confirmation.request", func(event interface{}) {
			logger.Debug("received tool.confirmation.request event")
			if confirmationEvent, ok := event.(events.ToolConfirmationRequest); ok {
				logger.Debug("auto-accepting tool confirmation", "tool", confirmationEvent.ToolName, "command", confirmationEvent.Command)
				cmd.Printf("Auto-accepting: %s - %s\n", confirmationEvent.ToolName, confirmationEvent.Command)
				response := events.ToolConfirmationResponse{
					ExecutionID: confirmationEvent.ExecutionID,
					Confirmed:   true,
				}
				logger.Debug("publishing tool confirmation response", "topic", response.Topic())
				eventBus.Publish(response.Topic(), response)
			} else {
				logger.Debug("event is not ToolConfirmationRequest", "event_type", fmt.Sprintf("%T", event))
			}
		})

		// Auto-approve diff confirmations
		logger.Debug("subscribing to user.confirmation.request events")
		eventBus.Subscribe("user.confirmation.request", func(event interface{}) {
			logger.Debug("received user.confirmation.request event")
			if confirmEvent, ok := event.(events.UserConfirmationRequest); ok {
				logger.Debug("auto-accepting user confirmation", "content_type", confirmEvent.ContentType, "file_path", confirmEvent.FilePath)
				cmd.Printf("Auto-accepting %s: %s\n", confirmEvent.ContentType, confirmEvent.FilePath)
				response := events.UserConfirmationResponse{
					ExecutionID: confirmEvent.ExecutionID,
					Confirmed:   true,
				}
				logger.Debug("publishing user confirmation response", "topic", response.Topic())
				eventBus.Publish(response.Topic(), response)
			} else {
				logger.Debug("event is not UserConfirmationRequest", "event_type", fmt.Sprintf("%T", event))
			}
		})
	}

	// Start chat with Genie
	logger.Debug("starting chat with Genie")
	err := g.Chat(context.Background(), message)
	if err != nil {
		return fmt.Errorf("failed to start chat: %w", err)
	}

	logger.Debug("chat started, waiting for completion")

	// Wait for completion (longer timeout for tool operations)
	timeout := 60 * time.Second
	if acceptAll {
		timeout = 120 * time.Second // Even longer for automated operations
	}

	logger.Debug("waiting for completion", "timeout", timeout)

	select {
	case err := <-done:
		logger.Debug("received completion signal", "error", err)
		return err
	case <-time.After(timeout):
		logger.Debug("timeout reached")
		return fmt.Errorf("timeout waiting for response")
	}
}

