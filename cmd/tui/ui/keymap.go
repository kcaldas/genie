package ui

import (
	"charm.land/bubbles/v2/key"
	"github.com/kcaldas/genie/cmd/tui/help"
)

// Keymap holds every binding the root model dispatches on. It is also the
// source for the help page, so what the page says is what is bound.
type Keymap struct {
	Help        key.Binding
	Debug       key.Binding
	Write       key.Binding
	Zoom        key.Binding
	Context     key.Binding
	NextPanel   key.Binding
	PersonaNext key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	LineUp      key.Binding
	LineDown    key.Binding
	Top         key.Binding
	Bottom      key.Binding
	Quit        key.Binding
	CopyLast    key.Binding
	CopyAll     key.Binding

	// Prompt keys, handled by the input model; listed for the help page.
	Newline        key.Binding
	HistorySearch  key.Binding
	ExternalEditor key.Binding
	Undo           key.Binding
	Redo           key.Binding
	Palette        key.Binding
	FilePicker     key.Binding
}

// DefaultKeymap is the shipped set of shortcuts.
func DefaultKeymap() Keymap {
	return Keymap{
		Help:        key.NewBinding(key.WithKeys("f1"), key.WithHelp("F1", "Show help dialog")),
		Debug:       key.NewBinding(key.WithKeys("f12"), key.WithHelp("F12", "Toggle debug panel visibility")),
		Write:       key.NewBinding(key.WithKeys("f4"), key.WithHelp("F4", "Open text area zoom for composing longer messages")),
		Zoom:        key.NewBinding(key.WithKeys("ctrl+z"), key.WithHelp("Ctrl+Z", "Toggle right panel zoom (confirmations, debug, etc.)")),
		Context:     key.NewBinding(key.WithKeys("ctrl+/", "ctrl+_"), key.WithHelp("Ctrl+/", "Show LLM context viewer")),
		NextPanel:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "Focus on next panel")),
		PersonaNext: key.NewBinding(key.WithKeys("ctrl+p", "shift+tab"), key.WithHelp("Ctrl+P / Shift+Tab", "Cycle to next persona in cycle list")),
		PageUp:      key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("PgUp / Ctrl+U", "Scroll up one page")),
		PageDown:    key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("PgDn / Ctrl+D", "Scroll down one page")),
		LineUp:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑ / k", "Scroll up (when messages or a panel has focus)")),
		LineDown:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓ / j", "Scroll down (when messages or a panel has focus)")),
		Top:         key.NewBinding(key.WithKeys("home"), key.WithHelp("Home", "Scroll to top")),
		Bottom:      key.NewBinding(key.WithKeys("end"), key.WithHelp("End", "Scroll to bottom")),
		Quit:        key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("Ctrl+C", "Exit application (on an empty prompt)")),
		CopyLast:    key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "Copy the last message (messages focused)")),
		CopyAll:     key.NewBinding(key.WithKeys("Y"), key.WithHelp("Y", "Copy all messages (messages focused)")),

		Newline:        key.NewBinding(key.WithKeys("shift+enter", "alt+enter", "ctrl+j"), key.WithHelp("Shift+Enter / Alt+Enter / Ctrl+J", "New line in the prompt")),
		HistorySearch:  key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("Ctrl+R", "Search prompt history")),
		ExternalEditor: key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("Ctrl+G", "Edit the prompt in $EDITOR")),
		Undo:           key.NewBinding(key.WithKeys("alt+z", "ctrl+_"), key.WithHelp("Alt+Z / Ctrl+_", "Undo in the prompt")),
		Redo:           key.NewBinding(key.WithKeys("alt+shift+z", "ctrl+y"), key.WithHelp("Alt+Shift+Z / Ctrl+Y", "Redo in the prompt")),
		Palette:        key.NewBinding(key.WithKeys("/"), key.WithHelp("/ (empty prompt)", "Command palette")),
		FilePicker:     key.NewBinding(key.WithKeys("@"), key.WithHelp("@", "File picker")),
	}
}

// Shortcuts lists the bindings for the help page. Command names the ':'
// command a key stands in for, so the page can say "same as :write".
func (k Keymap) Shortcuts() []help.Shortcut {
	entry := func(b key.Binding, command string) help.Shortcut {
		return help.Shortcut{Key: b.Help().Key, Description: b.Help().Desc, Command: command}
	}
	return []help.Shortcut{
		entry(k.Help, "help"),
		entry(k.Debug, ""),
		entry(k.Write, "write"),
		entry(k.Zoom, ""),
		entry(k.Context, "context"),
		entry(k.NextPanel, ""),
		entry(k.PersonaNext, ""),
		entry(k.PageUp, ""),
		entry(k.PageDown, ""),
		entry(k.LineUp, ""),
		entry(k.LineDown, ""),
		entry(k.Top, ""),
		entry(k.Bottom, ""),
		entry(k.Quit, ""),
		entry(k.CopyLast, ""),
		entry(k.CopyAll, ""),
		entry(k.Newline, ""),
		entry(k.HistorySearch, ""),
		entry(k.ExternalEditor, ""),
		entry(k.Undo, ""),
		entry(k.Redo, ""),
		entry(k.Palette, ""),
		entry(k.FilePicker, ""),
	}
}
