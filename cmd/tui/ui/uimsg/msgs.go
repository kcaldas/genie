// Package uimsg holds the messages controllers dispatch to the UI. They are
// plain values with no dependency on any UI toolkit, so controllers and
// their tests can use them without importing the rendering layer.
package uimsg

// ChatChanged tells the UI that the chat message list has changed and
// should be re-rendered from state.
type ChatChanged struct{}
