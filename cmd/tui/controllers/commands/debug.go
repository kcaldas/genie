package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/pkg/logging"
)

type DebugCommand struct {
	BaseCommand
	controller   *controllers.DebugController
	notification *controllers.ChatController
}

func NewDebugCommand(controller *controllers.DebugController, notification *controllers.ChatController) *DebugCommand {
	return &DebugCommand{
		BaseCommand: BaseCommand{
			Name:        "debug",
			Description: "Toggle debug logging on/off (use F12 to view debug panel)",
			Usage:       ":debug",
			Examples: []string{
				":debug",
			},
			Aliases:  []string{},
			Category: "Development",
		},
		controller:   controller,
		notification: notification,
	}
}

func (c *DebugCommand) Execute(args []string) error {
	// Toggle debug level via environment variable approach
	currentLevel := os.Getenv("GENIE_DEBUG_LEVEL")
	
	var newLevel string
	var status string
	
	if strings.ToLower(currentLevel) == "debug" {
		// Turn off debug logging
		newLevel = ""
		status = "disabled"
		os.Unsetenv("GENIE_DEBUG_LEVEL")
	} else {
		// Turn on debug logging
		newLevel = "debug"
		status = "enabled"  
		os.Setenv("GENIE_DEBUG_LEVEL", "debug")
	}
	
	// Apply the new log level to global logger
	c.controller.UpdateLogLevel(newLevel)
	
	// Show status message with file location
	debugFile := logging.GetDebugFilePath("genie-debug.log")
	
	message := fmt.Sprintf("Debug logging %s. File: %s. Use F12 to view debug panel or tail externally.", 
		status, debugFile)
	c.notification.AddSystemMessage(message)

	return nil
}
