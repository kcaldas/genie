package controllers

import (
	"sort"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/commands"
)

// Legacy Command struct for backward compatibility
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

// CommandWrapper wraps both old and new command types
type CommandWrapper struct {
	NewCommand commands.Command // New command interface
	OldCommand *Command         // Legacy command struct
}

func (w *CommandWrapper) GetName() string {
	if w.NewCommand != nil {
		return w.NewCommand.GetName()
	}
	return w.OldCommand.Name
}

func (w *CommandWrapper) GetDescription() string {
	if w.NewCommand != nil {
		return w.NewCommand.GetDescription()
	}
	return w.OldCommand.Description
}

func (w *CommandWrapper) GetUsage() string {
	if w.NewCommand != nil {
		return w.NewCommand.GetUsage()
	}
	return w.OldCommand.Usage
}

func (w *CommandWrapper) GetExamples() []string {
	if w.NewCommand != nil {
		return w.NewCommand.GetExamples()
	}
	return w.OldCommand.Examples
}

func (w *CommandWrapper) GetAliases() []string {
	if w.NewCommand != nil {
		return w.NewCommand.GetAliases()
	}
	return w.OldCommand.Aliases
}

func (w *CommandWrapper) GetCategory() string {
	if w.NewCommand != nil {
		return w.NewCommand.GetCategory()
	}
	return w.OldCommand.Category
}

func (w *CommandWrapper) IsHidden() bool {
	if w.NewCommand != nil {
		return w.NewCommand.IsHidden()
	}
	return w.OldCommand.Hidden
}

func (w *CommandWrapper) Execute(args []string) error {
	if w.NewCommand != nil {
		return w.NewCommand.Execute(args)
	}
	return w.OldCommand.Handler(args)
}

// CommandRegistry manages command registration and metadata
type CommandRegistry struct {
	commands       map[string]*CommandWrapper   // Primary name -> CommandWrapper
	aliasToCommand map[string]*CommandWrapper   // Alias -> CommandWrapper
	categories     map[string][]*CommandWrapper // Category -> CommandWrapper
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands:       make(map[string]*CommandWrapper),
		aliasToCommand: make(map[string]*CommandWrapper),
		categories:     make(map[string][]*CommandWrapper),
	}
}

// Register adds a legacy command to the registry
func (r *CommandRegistry) Register(cmd *Command) {
	wrapper := &CommandWrapper{OldCommand: cmd}
	r.registerWrapper(wrapper)
}

// RegisterNewCommand adds a new command interface to the registry
func (r *CommandRegistry) RegisterNewCommand(cmd commands.Command) {
	wrapper := &CommandWrapper{NewCommand: cmd}
	r.registerWrapper(wrapper)
}

// registerWrapper adds a CommandWrapper to the registry
func (r *CommandRegistry) registerWrapper(wrapper *CommandWrapper) {
	name := wrapper.GetName()
	aliases := wrapper.GetAliases()
	category := wrapper.GetCategory()
	
	// Register primary command
	r.commands[name] = wrapper
	
	// Register aliases
	for _, alias := range aliases {
		r.aliasToCommand[alias] = wrapper
	}
	
	// Register in category
	if category != "" {
		r.categories[category] = append(r.categories[category], wrapper)
	}
}

// GetCommand returns a command by name or alias
func (r *CommandRegistry) GetCommand(name string) *CommandWrapper {
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
func (r *CommandRegistry) GetAllCommands() []*CommandWrapper {
	var commands []*CommandWrapper
	for _, cmd := range r.commands {
		if !cmd.IsHidden() {
			commands = append(commands, cmd)
		}
	}
	
	// Sort by name for consistent output
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].GetName() < commands[j].GetName()
	})
	
	return commands
}

// GetCommandsByCategory returns commands grouped by category
func (r *CommandRegistry) GetCommandsByCategory() map[string][]*CommandWrapper {
	result := make(map[string][]*CommandWrapper)
	
	for category, commands := range r.categories {
		var visibleCommands []*CommandWrapper
		for _, cmd := range commands {
			if !cmd.IsHidden() {
				visibleCommands = append(visibleCommands, cmd)
			}
		}
		
		if len(visibleCommands) > 0 {
			// Sort commands within category
			sort.Slice(visibleCommands, func(i, j int) bool {
				return visibleCommands[i].GetName() < visibleCommands[j].GetName()
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
		if !r.commands[name].IsHidden() {
			names = append(names, name)
		}
	}
	
	// Add aliases
	for alias := range r.aliasToCommand {
		if !r.aliasToCommand[alias].IsHidden() {
			names = append(names, alias)
		}
	}
	
	sort.Strings(names)
	return names
}

// SearchCommands finds commands matching the query in name or description
func (r *CommandRegistry) SearchCommands(query string) []*CommandWrapper {
	query = strings.ToLower(query)
	var matches []*CommandWrapper
	
	for _, cmd := range r.commands {
		if cmd.IsHidden() {
			continue
		}
		
		// Check name match
		if strings.Contains(strings.ToLower(cmd.GetName()), query) {
			matches = append(matches, cmd)
			continue
		}
		
		// Check alias match
		for _, alias := range cmd.GetAliases() {
			if strings.Contains(strings.ToLower(alias), query) {
				matches = append(matches, cmd)
				break
			}
		}
		
		// Check description match
		if strings.Contains(strings.ToLower(cmd.GetDescription()), query) {
			matches = append(matches, cmd)
		}
	}
	
	// Remove duplicates and sort
	seen := make(map[string]bool)
	var uniqueMatches []*CommandWrapper
	for _, cmd := range matches {
		if !seen[cmd.GetName()] {
			seen[cmd.GetName()] = true
			uniqueMatches = append(uniqueMatches, cmd)
		}
	}
	
	sort.Slice(uniqueMatches, func(i, j int) bool {
		return uniqueMatches[i].GetName() < uniqueMatches[j].GetName()
	})
	
	return uniqueMatches
}

// GetAliases returns all aliases for a command
func (r *CommandRegistry) GetAliases(commandName string) []string {
	if cmd, ok := r.commands[commandName]; ok {
		return cmd.GetAliases()
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