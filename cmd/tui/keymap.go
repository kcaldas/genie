package tui

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/help"
)

// KeyAction represents an action that can be triggered by a key
type KeyAction struct {
	Type        string       // "command" or "function"
	CommandName string       // For "command" type - name of command to execute
	Function    func() error // For "function" type - direct function to call
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

// Shortcuts converts the keymap into the toolkit-neutral form the help
// renderer consumes.
func (k *Keymap) Shortcuts() []help.Shortcut {
	out := make([]help.Shortcut, 0, len(k.entries))
	for _, e := range k.entries {
		s := help.Shortcut{Key: e.KeyName(), Description: e.Description}
		if e.Action.Type == "command" {
			s.Command = e.Action.CommandName
		}
		out = append(out, s)
	}
	return out
}

// KeyName renders the entry's key and modifier as the help page shows it.
func (e KeymapEntry) KeyName() string {
	var parts []string
	if e.Mod != gocui.ModNone {
		parts = append(parts, fmt.Sprintf("Mod(%d)", e.Mod))
	}
	parts = append(parts, gocuiKeyName(e.Key))
	return strings.Join(parts, "+")
}

var gocuiKeyNames = map[gocui.Key]string{
	gocui.KeyF1: "F1", gocui.KeyF2: "F2", gocui.KeyF3: "F3", gocui.KeyF4: "F4",
	gocui.KeyF5: "F5", gocui.KeyF6: "F6", gocui.KeyF7: "F7", gocui.KeyF8: "F8",
	gocui.KeyF9: "F9", gocui.KeyF10: "F10", gocui.KeyF11: "F11", gocui.KeyF12: "F12",
	gocui.KeyCtrlC: "Ctrl+C", gocui.KeyPgup: "PgUp", gocui.KeyPgdn: "PgDn",
	gocui.KeyArrowUp: "↑", gocui.KeyArrowDown: "↓", gocui.KeyArrowLeft: "←", gocui.KeyArrowRight: "→",
	gocui.KeyHome: "Home", gocui.KeyEnd: "End", gocui.KeyEnter: "Enter", gocui.KeySpace: "Space",
	gocui.KeyTab: "Tab", gocui.KeyEsc: "Esc", gocui.KeyBackspace: "Backspace", gocui.KeyDelete: "Del",
	gocui.KeyInsert: "Ins", gocui.MouseWheelUp: "Mouse Wheel Up", gocui.MouseWheelDown: "Mouse Wheel Down",
	gocui.MouseLeft: "Left Click", gocui.MouseMiddle: "Middle Click", gocui.MouseRight: "Right Click",
}

func gocuiKeyName(key gocui.Key) string {
	if name, ok := gocuiKeyNames[key]; ok {
		return name
	}
	if key >= 32 && key <= 126 {
		return strings.ToUpper(string(rune(key)))
	}
	return fmt.Sprintf("Key(%d)", key)
}
