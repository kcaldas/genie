package controllers

import (
	"sort"
	"strings"
)

// Command represents a slash command with full metadata
type Command struct {
	Name        string      // Primary command name (without /)
	Description string      // Short description for help
	Usage       string      // Usage syntax (e.g., "/config <setting> <value>")
	Examples    []string    // Usage examples
	Aliases     []string    // Alternative names
	Category    string      // Command category for grouping
	Handler     CommandFunc // Function to execute
	Hidden      bool        // Hide from help (for internal commands)
}

// CommandRegistry manages command registration and metadata
type CommandRegistry struct {
	commands       map[string]*Command // Primary name -> Command
	aliasToCommand map[string]*Command // Alias -> Command
	categories     map[string][]*Command // Category -> Commands
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands:       make(map[string]*Command),
		aliasToCommand: make(map[string]*Command),
		categories:     make(map[string][]*Command),
	}
}

// Register adds a command to the registry
func (r *CommandRegistry) Register(cmd *Command) {
	// Register primary command
	r.commands[cmd.Name] = cmd
	
	// Register aliases
	for _, alias := range cmd.Aliases {
		r.aliasToCommand[alias] = cmd
	}
	
	// Register in category
	if cmd.Category != "" {
		r.categories[cmd.Category] = append(r.categories[cmd.Category], cmd)
	}
}

// GetCommand returns a command by name or alias
func (r *CommandRegistry) GetCommand(name string) *Command {
	// Try primary name first
	if cmd, ok := r.commands[name]; ok {
		return cmd
	}
	
	// Try aliases
	if cmd, ok := r.aliasToCommand[name]; ok {
		return cmd
	}
	
	return nil
}

// GetAllCommands returns all registered commands (excluding hidden)
func (r *CommandRegistry) GetAllCommands() []*Command {
	var commands []*Command
	for _, cmd := range r.commands {
		if !cmd.Hidden {
			commands = append(commands, cmd)
		}
	}
	
	// Sort by name for consistent output
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})
	
	return commands
}

// GetCommandsByCategory returns commands grouped by category
func (r *CommandRegistry) GetCommandsByCategory() map[string][]*Command {
	result := make(map[string][]*Command)
	
	for category, commands := range r.categories {
		var visibleCommands []*Command
		for _, cmd := range commands {
			if !cmd.Hidden {
				visibleCommands = append(visibleCommands, cmd)
			}
		}
		
		if len(visibleCommands) > 0 {
			// Sort commands within category
			sort.Slice(visibleCommands, func(i, j int) bool {
				return visibleCommands[i].Name < visibleCommands[j].Name
			})
			result[category] = visibleCommands
		}
	}
	
	return result
}

// GetCommandNames returns all command names (including aliases)
func (r *CommandRegistry) GetCommandNames() []string {
	var names []string
	
	// Add primary names
	for name := range r.commands {
		if !r.commands[name].Hidden {
			names = append(names, name)
		}
	}
	
	// Add aliases
	for alias := range r.aliasToCommand {
		if !r.aliasToCommand[alias].Hidden {
			names = append(names, alias)
		}
	}
	
	sort.Strings(names)
	return names
}

// SearchCommands finds commands matching the query in name or description
func (r *CommandRegistry) SearchCommands(query string) []*Command {
	query = strings.ToLower(query)
	var matches []*Command
	
	for _, cmd := range r.commands {
		if cmd.Hidden {
			continue
		}
		
		// Check name match
		if strings.Contains(strings.ToLower(cmd.Name), query) {
			matches = append(matches, cmd)
			continue
		}
		
		// Check alias match
		for _, alias := range cmd.Aliases {
			if strings.Contains(strings.ToLower(alias), query) {
				matches = append(matches, cmd)
				break
			}
		}
		
		// Check description match
		if strings.Contains(strings.ToLower(cmd.Description), query) {
			matches = append(matches, cmd)
		}
	}
	
	// Remove duplicates and sort
	seen := make(map[string]bool)
	var uniqueMatches []*Command
	for _, cmd := range matches {
		if !seen[cmd.Name] {
			seen[cmd.Name] = true
			uniqueMatches = append(uniqueMatches, cmd)
		}
	}
	
	sort.Slice(uniqueMatches, func(i, j int) bool {
		return uniqueMatches[i].Name < uniqueMatches[j].Name
	})
	
	return uniqueMatches
}

// GetAliases returns all aliases for a command
func (r *CommandRegistry) GetAliases(commandName string) []string {
	if cmd, ok := r.commands[commandName]; ok {
		return cmd.Aliases
	}
	return nil
}

// GetCategories returns all category names
func (r *CommandRegistry) GetCategories() []string {
	var categories []string
	for category := range r.categories {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories
}