package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type PersonaCommand struct {
	BaseCommand
	notification    types.Notification
	genieService    genie.Genie
	commandEventBus *events.CommandEventBus
	configManager   *helpers.ConfigManager
}

func NewPersonaCommand(notification types.Notification, genieService genie.Genie, commandEventBus *events.CommandEventBus, configManager *helpers.ConfigManager) *PersonaCommand {
	return &PersonaCommand{
		BaseCommand: BaseCommand{
			Name:        "persona",
			Description: "Manage personas",
			Usage:       ":persona list (or :p -l) | :persona swap <persona_id> (or :p -s <persona_id>) | :persona cycle add/remove <persona_id> | :persona next",
			Examples: []string{
				":persona list",
				":p -l",
				":persona swap engineer",
				":p -s product_owner",
				":persona cycle add engineer",
				":persona cycle remove engineer",
				":persona next",
			},
			Aliases:  []string{"p"},
			Category: "Persona",
		},
		notification:    notification,
		genieService:    genieService,
		commandEventBus: commandEventBus,
		configManager:   configManager,
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
	case "cycle":
		if len(args) < 3 {
			return fmt.Errorf("cycle requires an action and persona ID. Usage: :persona cycle add/remove <persona_id>")
		}
		return c.executeCycle(args[1], args[2])
	case "next":
		return c.executeCycleNext()
	default:
		return fmt.Errorf("unknown subcommand '%s'. Available: list, swap, cycle, next", subcommand)
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

func (c *PersonaCommand) executeCycle(action, personaId string) error {
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
	
	switch action {
	case "add":
		return c.executeCycleAdd(personaId, foundPersona.GetName())
	case "remove":
		return c.executeCycleRemove(personaId, foundPersona.GetName())
	default:
		return fmt.Errorf("unknown cycle action '%s'. Available: add, remove", action)
	}
}

func (c *PersonaCommand) executeCycleAdd(personaId, personaName string) error {
	config := c.configManager.GetConfig()
	
	// Check if persona is already in the cycle list
	for _, existingId := range config.PersonaCycleList {
		if existingId == personaId {
			c.notification.AddSystemMessage(fmt.Sprintf("Persona '%s' (%s) is already in the cycle list", 
				personaId, personaName))
			return nil
		}
	}
	
	// Add persona to cycle list
	config.PersonaCycleList = append(config.PersonaCycleList, personaId)
	
	// Save the updated config
	if err := c.configManager.SaveWithScope(config, false); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	
	c.notification.AddSystemMessage(fmt.Sprintf("Added persona '%s' (%s) to cycle list", 
		personaId, personaName))
	
	return nil
}

func (c *PersonaCommand) executeCycleRemove(personaId, personaName string) error {
	config := c.configManager.GetConfig()
	
	// Find and remove persona from cycle list
	var newCycleList []string
	found := false
	for _, existingId := range config.PersonaCycleList {
		if existingId == personaId {
			found = true
			continue // Skip this persona (remove it)
		}
		newCycleList = append(newCycleList, existingId)
	}
	
	if !found {
		c.notification.AddSystemMessage(fmt.Sprintf("Persona '%s' (%s) is not in the cycle list", 
			personaId, personaName))
		return nil
	}
	
	// Update config with new cycle list
	config.PersonaCycleList = newCycleList
	
	// Save the updated config
	if err := c.configManager.SaveWithScope(config, false); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	
	c.notification.AddSystemMessage(fmt.Sprintf("Removed persona '%s' (%s) from cycle list", 
		personaId, personaName))
	
	return nil
}

func (c *PersonaCommand) executeCycleNext() error {
	config := c.configManager.GetConfig()
	
	// Check if cycle list is empty
	if len(config.PersonaCycleList) == 0 {
		c.notification.AddSystemMessage("Persona cycle list is empty. Use ':persona cycle add <persona_id>' to add personas to the cycle.")
		return nil
	}
	
	// Get current session to find current persona
	session, err := c.genieService.GetSession()
	if err != nil {
		return fmt.Errorf("failed to get current session: %w", err)
	}
	
	currentPersona := session.GetPersona()
	var currentPersonaId string
	if currentPersona != nil {
		currentPersonaId = currentPersona.GetID()
	}
	
	// Find the next persona in the cycle
	var nextPersonaId string
	if currentPersonaId == "" {
		// No current persona, start with the first in the cycle
		nextPersonaId = config.PersonaCycleList[0]
	} else {
		// Find current persona in cycle list and get next one
		found := false
		for i, personaId := range config.PersonaCycleList {
			if personaId == currentPersonaId {
				// Found current persona, get next one (wrap around if at end)
				nextPersonaId = config.PersonaCycleList[(i+1)%len(config.PersonaCycleList)]
				found = true
				break
			}
		}
		
		if !found {
			// Current persona is not in cycle list, start with first persona
			nextPersonaId = config.PersonaCycleList[0]
		}
	}
	
	// Validate that the next persona still exists
	ctx := context.Background()
	personas, err := c.genieService.ListPersonas(ctx)
	if err != nil {
		return fmt.Errorf("failed to list personas: %w", err)
	}
	
	var foundPersona genie.Persona
	for _, persona := range personas {
		if persona.GetID() == nextPersonaId {
			foundPersona = persona
			break
		}
	}
	
	if foundPersona == nil {
		// Persona no longer exists, remove it from cycle list and try again
		c.notification.AddSystemMessage(fmt.Sprintf("Persona '%s' no longer exists, removing from cycle list", nextPersonaId))
		
		// Remove the invalid persona from cycle list
		var newCycleList []string
		for _, personaId := range config.PersonaCycleList {
			if personaId != nextPersonaId {
				newCycleList = append(newCycleList, personaId)
			}
		}
		config.PersonaCycleList = newCycleList
		
		// Save updated config
		if err := c.configManager.SaveWithScope(config, false); err != nil {
			return fmt.Errorf("failed to save config after removing invalid persona: %w", err)
		}
		
		// Try again with updated list
		if len(config.PersonaCycleList) == 0 {
			c.notification.AddSystemMessage("Persona cycle list is now empty.")
			return nil
		}
		return c.executeCycleNext()
	}
	
	// Switch to the next persona (same logic as executeSwap)
	session.SetPersona(foundPersona)
	
	// Provide success feedback
	c.notification.AddSystemMessage(fmt.Sprintf("Cycled to persona '%s' (%s)", 
		nextPersonaId, foundPersona.GetName()))
	
	// Emit persona change event to update UI title
	c.commandEventBus.Emit("persona.changed", map[string]interface{}{
		"name": foundPersona.GetName(),
	})
	
	return nil
}