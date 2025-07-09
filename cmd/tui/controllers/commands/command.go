package commands

import (
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
)

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

// CommandContext provides access to app components for commands
type CommandContext struct {
	StateAccessor          types.IStateAccessor
	GuiCommon              types.IGuiCommon
	ClipboardHelper        ClipboardHelper
	ConfigHelper           ConfigHelper
	ShowLLMContextViewer   func() error
	SetCurrentView         func(viewName string) error
	ChatController         ChatControllerInterface
	ToggleHelpInTextViewer func() error            // New: Toggle help in text viewer panel
	Exit                   func() error            // New: Exit the application
	CommandEventBus        *events.CommandEventBus // New: Event bus for emitting UI events
}

// Helper interfaces to avoid circular dependencies
type ClipboardHelper interface {
	Copy(text string) error
	Paste() (string, error)
	IsAvailable() bool
}

type ConfigHelper interface {
	Save(config *types.Config) error
	Load() (*types.Config, error)
	GetDefaultConfig() *types.Config
}

type ChatControllerInterface interface {
	ClearConversation() error
	GetConversationHistory() []types.Message
}


