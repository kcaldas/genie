package commands

import (
	"sort"
	"strings"
)

// CommandWrapper wraps both old and new command types
type CommandWrapper struct {
	NewCommand Command // New command interface
}

func (w *CommandWrapper) GetName() string {
	return w.NewCommand.GetName()
}

func (w *CommandWrapper) GetDescription() string {
	return w.NewCommand.GetDescription()
}

func (w *CommandWrapper) GetUsage() string {
	return w.NewCommand.GetUsage()
}

func (w *CommandWrapper) GetExamples() []string {
	return w.NewCommand.GetExamples()
}

func (w *CommandWrapper) GetAliases() []string {
	return w.NewCommand.GetAliases()
}

func (w *CommandWrapper) GetCategory() string {
	return w.NewCommand.GetCategory()
}

func (w *CommandWrapper) IsHidden() bool {
	return w.NewCommand.IsHidden()
}

func (w *CommandWrapper) Execute(args []string) error {
	return w.NewCommand.Execute(args)
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

// RegisterNewCommand adds a new command interface to the registry
func (r *CommandRegistry) RegisterNewCommand(cmd Command) {
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

