package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type PersonaCommand struct {
	BaseCommand
	notification    types.Notification
	genieService    genie.Genie
	commandEventBus *events.CommandEventBus
}

func NewPersonaCommand(notification types.Notification, genieService genie.Genie, commandEventBus *events.CommandEventBus) *PersonaCommand {
	return &PersonaCommand{
		BaseCommand: BaseCommand{
			Name:        "persona",
			Description: "Manage personas",
			Usage:       ":persona list (or :p -l) | :persona swap <persona_id> (or :p -s <persona_id>)",
			Examples: []string{
				":persona list",
				":p -l",
				":persona swap engineer",
				":p -s product_owner",
			},
			Aliases:  []string{"p"},
			Category: "Persona",
		},
		notification:    notification,
		genieService:    genieService,
		commandEventBus: commandEventBus,
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
	case "swap", "-s":
		if len(args) < 2 {
			return fmt.Errorf("swap requires a persona ID. Usage: :persona swap <persona_id>")
		}
		return c.executeSwap(args[1])
	default:
		return fmt.Errorf("unknown subcommand '%s'. Available: list, swap", subcommand)
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

func (c *PersonaCommand) executeSwap(personaId string) error {
	ctx := context.Background()
	
	// First, validate that the persona exists
	personas, err := c.genieService.ListPersonas(ctx)
	if err != nil {
		return fmt.Errorf("failed to list personas for validation: %w", err)
	}
	
	// Check if the requested persona exists
	var foundPersona genie.Persona
	for _, persona := range personas {
		if persona.GetID() == personaId {
			foundPersona = persona
			break
		}
	}
	
	if foundPersona == nil {
		// Build helpful error message with available personas
		var availableIds []string
		for _, persona := range personas {
			availableIds = append(availableIds, persona.GetID())
		}
		return fmt.Errorf("persona '%s' not found. Available personas: %s", 
			personaId, strings.Join(availableIds, ", "))
	}
	
	// Get the current session
	session, err := c.genieService.GetSession()
	if err != nil {
		return fmt.Errorf("failed to get current session: %w", err)
	}
	
	// Get current persona for comparison
	currentPersona := session.GetPersona()
	
	// Check if we're already using this persona
	if currentPersona != nil && currentPersona.GetID() == personaId {
		c.notification.AddSystemMessage(fmt.Sprintf("Already using persona '%s' (%s)", 
			personaId, foundPersona.GetName()))
		return nil
	}
	
	// Update the session with the new persona
	session.SetPersona(foundPersona)
	
	// Provide success feedback
	c.notification.AddSystemMessage(fmt.Sprintf("Switched to persona '%s' (%s) from %s", 
		personaId, foundPersona.GetName(), foundPersona.GetSource()))
	
	// Emit persona change event to update UI title
	c.commandEventBus.Emit("persona.changed", map[string]interface{}{
		"name": foundPersona.GetName(),
	})
	
	return nil
}