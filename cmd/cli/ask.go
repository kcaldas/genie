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

// truncateContent truncates content to maxLength and adds "..." if truncated
func truncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "..."
}

// constructMessage builds the message from stdin and/or command arguments
func constructMessage(args []string) (string, error) {
	var parts []string
	
	// Read stdin if available
	if hasStdinInput() {
		stdinContent, err := readStdinInput()
		if err != nil {
			return "", err
		}
		if stdinContent != "" {
			parts = append(parts, stdinContent)
		}
	}
	
	// Add command arguments if provided
	if len(args) > 0 {
		argsText := strings.Join(args, " ")
		parts = append(parts, argsText)
	}
	
	if len(parts) == 0 {
		return "", fmt.Errorf("no input provided via stdin or arguments")
	}
	
	// Join parts with a separator to distinguish stdin content from prompt
	if len(parts) == 1 {
		return parts[0], nil
	}
	
	// When both stdin and args are present, format as: <stdin_content>\n\n<prompt>
	return strings.Join(parts, "\n\n"), nil
}

// validateAskArgs validates arguments for the ask command
// Allows zero args if stdin has data, otherwise requires at least one arg
func validateAskArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return nil // Args provided, always valid
	}
	
	// No args provided, check if stdin has data
	if hasStdinInput() {
		return nil // Stdin available, valid
	}
	
	return fmt.Errorf("requires at least 1 arg(s), only received %d. Use pipes like 'git diff | genie ask \"explain\"' or provide arguments directly", len(args))
}

// NewAskCommandWithGenie creates an ask command that uses a pre-initialized Genie instance
func NewAskCommandWithGenie(genieProvider func() (genie.Genie, *genie.Session)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask [message]",
		Short: "Ask the AI a question (supports piped input)",
		Long: `Ask the AI a question with optional piped input.

Examples:
  genie ask "What is the meaning of life?"
  git diff | genie ask "suggest a commit message"
  find . -name "*.go" | genie ask "what patterns do you see?"
  cat README.md | genie ask "summarize this"`,
		Args: validateAskArgs,
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
	// Construct message from stdin and/or arguments
	message, err := constructMessage(args)
	if err != nil {
		return fmt.Errorf("failed to construct message: %w", err)
	}

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
				logger.Info("🤖 Assistant", "response", resp.Response)
				fmt.Println(resp.Response)
				done <- nil // Success
			}
		} else {
			logger.Debug("event is not ChatResponseEvent", "event_type", fmt.Sprintf("%T", event))
		}
	})

	// Subscribe to conversation flow events for TUI-like logging
	eventBus.Subscribe("chat.started", func(event interface{}) {
		if startEvent, ok := event.(events.ChatStartedEvent); ok {
			logger.Info("💬 User", "message", startEvent.Message)
		}
	})

	eventBus.Subscribe("tool.executed", func(event interface{}) {
		if execEvent, ok := event.(events.ToolExecutedEvent); ok {
			// Truncate result for display
			truncatedResult := truncateContent(fmt.Sprintf("%v", execEvent.Result), 200)
			logger.Info("✅ Tool Result", "tool", execEvent.ToolName, "message", execEvent.Message, "result", truncatedResult)
		}
	})

	eventBus.Subscribe("tool.call.message", func(event interface{}) {
		if msgEvent, ok := event.(events.ToolCallMessageEvent); ok {
			logger.Info("📝 Tool Message", "tool", msgEvent.ToolName, "message", msgEvent.Message)
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
	err = g.Chat(context.Background(), message)
	if err != nil {
		return fmt.Errorf("failed to start chat: %w", err)
	}

	logger.Debug("chat started, waiting for completion")

	// Wait for completion (longer timeout for tool operations)
	timeout := 60 * time.Second
	if acceptAll {
		timeout = 30 * 60 * time.Second // 30 minutes for automated operations
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
