package commands


// Command represents a command that can be executed
type Command interface {
	// Metadata
	GetName() string
	GetDescription() string
	GetUsage() string
	GetExamples() []string
	GetAliases() []string
	GetCategory() string
	IsHidden() bool
	GetShortcuts() []string

	// Execution
	Execute(args []string) error
}

// BaseCommand provides common functionality for all commands
type BaseCommand struct {
	Name        string
	Description string
	Usage       string
	Examples    []string
	Aliases     []string
	Category    string
	Hidden      bool
	Shortcuts   []string
}

func (c *BaseCommand) GetName() string        { return c.Name }
func (c *BaseCommand) GetDescription() string { return c.Description }
func (c *BaseCommand) GetUsage() string       { return c.Usage }
func (c *BaseCommand) GetExamples() []string  { return c.Examples }
func (c *BaseCommand) GetAliases() []string   { return c.Aliases }
func (c *BaseCommand) GetCategory() string    { return c.Category }
func (c *BaseCommand) IsHidden() bool         { return c.Hidden }
func (c *BaseCommand) GetShortcuts() []string { return c.Shortcuts }

// DebugControllerInterface interface for debug functionality
type DebugControllerInterface interface {
	AddDebugMessage(message string)
	ClearDebugMessages()
	GetDebugMessages() []string
	IsDebugMode() bool
	SetDebugMode(enabled bool)
}
