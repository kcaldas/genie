package commands

import (
	"fmt"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type StatusCommand struct {
	BaseCommand
	notification types.Notification
	genieService genie.Genie
}

func NewStatusCommand(notification types.Notification, genieService genie.Genie) *StatusCommand {
	return &StatusCommand{
		BaseCommand: BaseCommand{
			Name:        "status",
			Description: "Show the current status of the AI backend",
			Usage:       ":status",
			Examples: []string{
				":status",
			},
			Aliases:  []string{"st"},
			Category: "System",
		},
		notification: notification,
		genieService: genieService,
	}
}

func (c *StatusCommand) Execute(args []string) error {
	status := c.genieService.GetStatus()
	
	// Format the status message
	var statusIcon string
	if status.Connected {
		statusIcon = "✓"
	} else {
		statusIcon = "✗"
	}
	
	message := fmt.Sprintf("%s Backend: %s | %s", statusIcon, status.Backend, status.Message)
	
	// Add as system message
	c.notification.AddSystemMessage(message)
	
	return nil
}