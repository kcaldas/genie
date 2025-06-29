package commands

import (
	"github.com/kcaldas/genie/cmd/tui2/types"
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
}

func (c *BaseCommand) GetName() string        { return c.Name }
func (c *BaseCommand) GetDescription() string { return c.Description }
func (c *BaseCommand) GetUsage() string       { return c.Usage }
func (c *BaseCommand) GetExamples() []string  { return c.Examples }
func (c *BaseCommand) GetAliases() []string   { return c.Aliases }
func (c *BaseCommand) GetCategory() string    { return c.Category }
func (c *BaseCommand) IsHidden() bool         { return c.Hidden }

// CommandContext provides access to app components for commands
type CommandContext struct {
	StateAccessor    types.IStateAccessor
	GuiCommon        types.IGuiCommon
	ClipboardHelper  ClipboardHelper
	ConfigHelper     ConfigHelper
	RefreshUI        func() error
	ShowHelpDialog   func(category string) error
	SetCurrentView   func(viewName string) error
	ChatController   ChatController
	DebugComponent   DebugComponent
	LayoutManager    LayoutManager
	MessageFormatter MessageFormatter
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
}

type ChatController interface {
	ClearConversation() error
	GetConversationHistory() []types.Message
}

type DebugComponent interface {
	ToggleVisibility()
	IsVisible() bool
}

type LayoutManager interface {
	GetAvailablePanels() []string
}

type MessageFormatter interface {
	// Add methods as needed
}