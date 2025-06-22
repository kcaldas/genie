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
	return &cobra.Command{
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
	
	// Start chat with Genie
	err = g.Chat(context.Background(), sessionID, message)
	if err != nil {
		return fmt.Errorf("failed to start chat: %w", err)
	}
	
	// Wait for response
	select {
	case resp := <-responseChan:
		if resp.Error != nil {
			return fmt.Errorf("chat failed: %w", resp.Error)
		}
		cmd.Println(resp.Response)
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for response")
	}
}
