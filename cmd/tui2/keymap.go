package tui2

import (
	"github.com/awesome-gocui/gocui"
)

// KeyAction represents an action that can be triggered by a key
type KeyAction struct {
	Type        string        // "command" or "function"
	CommandName string        // For "command" type - name of command to execute
	Function    func() error  // For "function" type - direct function to call
}

// KeymapEntry represents a single keybinding in the keymap
type KeymapEntry struct {
	Key         gocui.Key      // The key to bind
	Mod         gocui.Modifier // Key modifier (Ctrl, Alt, etc.)
	Action      KeyAction      // What action to perform
	Description string         // Human-readable description
}

// Keymap manages the application's keybindings
type Keymap struct {
	entries []KeymapEntry
}

// NewKeymap creates a new empty keymap
func NewKeymap() *Keymap {
	return &Keymap{
		entries: make([]KeymapEntry, 0),
	}
}

// AddEntry adds a new keybinding entry to the keymap
func (k *Keymap) AddEntry(entry KeymapEntry) {
	k.entries = append(k.entries, entry)
}

// GetEntries returns all keymap entries
func (k *Keymap) GetEntries() []KeymapEntry {
	return k.entries
}

// CommandAction creates a KeyAction that executes a command
func CommandAction(commandName string) KeyAction {
	return KeyAction{
		Type:        "command",
		CommandName: commandName,
	}
}

// FunctionAction creates a KeyAction that calls a function directly
func FunctionAction(fn func() error) KeyAction {
	return KeyAction{
		Type:     "function",
		Function: fn,
	}
}