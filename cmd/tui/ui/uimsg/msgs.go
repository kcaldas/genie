// Package uimsg holds the messages controllers dispatch to the UI. They are
// plain values with no dependency on any UI toolkit, so controllers and
// their tests can use them without importing the rendering layer.
package uimsg

import "github.com/kcaldas/genie/cmd/tui/types"

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
