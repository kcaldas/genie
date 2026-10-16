// Package uimsg holds the messages controllers dispatch to the UI. They are
// plain values with no dependency on any UI toolkit, so controllers and
// their tests can use them without importing the rendering layer.
package uimsg

import (
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
)

// ChatChanged tells the UI that the chat message list has changed and
// should be re-rendered from state.
type ChatChanged struct{}

// ChatStreaming tells the UI that the message with this ID received
// another streamed chunk. Only that message needs re-rendering.
type ChatStreaming struct{ ID int64 }

// PersonaChanged carries the display name of the newly active persona.
type PersonaChanged struct{ Name string }

// ThemeChanged carries the config after a theme or display setting change.
type ThemeChanged struct{ Config *types.Config }

// RequestStarted reports a new chat request; Active counts requests in
// flight including this one.
type RequestStarted struct{ Active int }

// RequestFinished reports a chat request ending; Last is true when no
// request remains in flight.
type RequestFinished struct{ Last bool }

// TokenCount reports tokens consumed by the current request.
type TokenCount struct{ Tokens int32 }

// CursorChanged toggles the terminal cursor.
type CursorChanged struct{ Visible bool }

// MouseChanged toggles mouse capture.
type MouseChanged struct{ Enabled bool }

// Quit asks the UI to exit.
type Quit struct{}

// PanelAction says what to do with the right panel.
type PanelAction int

const (
	PanelShow PanelAction = iota
	PanelHide
	PanelToggle
	PanelToggleZoom
)

// RightPanel shows, hides, toggles or zooms a right-panel viewer. Title,
// Content and ContentType replace the viewer's content when non-empty.
type RightPanel struct {
	Mode        layout.RightMode
	Action      PanelAction
	Title       string
	Content     string
	ContentType string // "text", "markdown" or "diff"
}

// DebugChanged tells the UI the debug log in state changed.
type DebugChanged struct{}

// OpenContextViewer opens the LLM context dialog with these parts.
type OpenContextViewer struct{ Data map[string]string }

// ContextData refreshes the parts shown by an open context viewer.
type ContextData struct{ Data map[string]string }

// CloseContextViewer closes the LLM context dialog if open.
type CloseContextViewer struct{}

// ExpandPrompt grows the prompt to the full screen height and appends
// Content to it (the :write command and multi-line paste events).
type ExpandPrompt struct{ Content string }

// VimModeChanged reports the VimMode setting changing while running.
type VimModeChanged struct{ Enabled bool }

// ConfirmKind says which core event a confirmation answers.
type ConfirmKind int

const (
	ConfirmTool ConfirmKind = iota // tool.confirmation.*
	ConfirmUser                    // user.confirmation.*
)

// ConfirmationRequested opens a confirmation dialog. Prompt is the key
// legend ("1 - Yes | 2 - No"); Content is shown in the dialog body as
// ContentType "text", "markdown" or "diff". ToolName, when set, lets the
// user allow that tool for the rest of the session or permanently.
type ConfirmationRequested struct {
	Kind        ConfirmKind
	ExecutionID string
	ToolName    string
	Title       string
	Prompt      string
	Content     string
	ContentType string
}

// ConfirmScope says how far a "yes" reaches.
type ConfirmScope int

const (
	ScopeOnce    ConfirmScope = iota
	ScopeSession              // auto-accept this tool until genie exits
	ScopeAlways               // auto-accept this tool and save it to the config
)

// ConfirmationClosed closes the open confirmation dialog, if any, for
// example when the request was cancelled from elsewhere.
type ConfirmationClosed struct{}

// TopicConfirmationAnswered is the command-bus topic the UI emits with a
// ConfirmationAnswer when the user picks yes or no.
const TopicConfirmationAnswered = "confirmation.answered"

// ConfirmationAnswer is the payload of TopicConfirmationAnswered.
type ConfirmationAnswer struct {
	Kind        ConfirmKind
	ExecutionID string
	ToolName    string
	Confirmed   bool
	Scope       ConfirmScope
}

// QueueChanged reports how many messages wait to be sent after the
// running request finishes.
type QueueChanged struct{ Count int }

// Palette kinds the prompt can open.
const (
	PaletteCommands = "commands" // '/' at the start of an empty prompt
	PaletteFiles    = "files"    // '@' anywhere
)

// OpenPalette asks the UI to open the command palette or the file picker.
type OpenPalette struct{ Kind string }

// PaletteHistory lists past prompts (Ctrl+R).
const PaletteHistory = "history"

// OpenExternalEditor asks the UI to hand Text to $EDITOR and put the
// result back into the prompt.
type OpenExternalEditor struct{ Text string }

// ExternalEditDone returns from the external editor. Text is the edited
// content; Err is set when the editor failed, in which case Text is the
// original.
type ExternalEditDone struct {
	Text string
	Err  error
}

// ToggleDetails flips whether tool output and thinking are shown expanded.
type ToggleDetails struct{}

// TodosUpdated carries the agent's current todo list after a TodoWrite.
type TodosUpdated struct{ Items []presentation.TodoItem }

// ToolActivity is the agent's narration of what it is doing right now
// (a tool's display message); shown in the status bar while a request runs.
type ToolActivity struct{ Text string }

// PaletteModels lists the models the configured providers serve (:model, Alt+M).
const PaletteModels = "models"

// ModelChanged carries the provider and model the next turn will use.
type ModelChanged struct{ Provider, Model string }

// Label renders the pair as "provider/model".
func (m ModelChanged) Label() string {
	switch {
	case m.Provider != "" && m.Model != "":
		return m.Provider + "/" + m.Model
	case m.Provider != "":
		return m.Provider
	default:
		return m.Model
	}
}
