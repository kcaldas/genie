package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type PersonaCommand struct {
	BaseCommand
	notification types.Notification
	genieService genie.Genie
}

func NewPersonaCommand(notification types.Notification, genieService genie.Genie) *PersonaCommand {
	return &PersonaCommand{
		BaseCommand: BaseCommand{
			Name:        "persona",
			Description: "Manage personas",
			Usage:       ":persona list (or :p -l) - List all available personas",
			Examples: []string{
				":persona list",
				":p -l",
				":p list",
			},
			Aliases:  []string{"p"},
			Category: "Persona",
		},
		notification: notification,
		genieService: genieService,
	}
}

func (c *PersonaCommand) Execute(args []string) error {
	// Default to list if no arguments provided
	if len(args) == 0 {
		return c.executeList()
	}
	
	subcommand := args[0]
	
	switch subcommand {
	case "list", "-l", "ls":
		return c.executeList()
	default:
		return fmt.Errorf("unknown subcommand '%s'. Available: list, -l", subcommand)
	}
}

func (c *PersonaCommand) executeList() error {
	ctx := context.Background()
	personas, err := c.genieService.ListPersonas(ctx)
	if err != nil {
		return fmt.Errorf("failed to list personas: %w", err)
	}
	
	if len(personas) == 0 {
		c.notification.AddSystemMessage("No personas found.")
		return nil
	}
	
	// Build the personas list message
	var builder strings.Builder
	builder.WriteString("Available personas:\n")
	
	for _, persona := range personas {
		builder.WriteString(fmt.Sprintf("  • %s (%s) - %s\n", 
			persona.GetID(), 
			persona.GetSource(), 
			persona.GetName()))
	}
	
	c.notification.AddSystemMessage(builder.String())
	return nil
}