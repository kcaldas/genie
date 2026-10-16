package uimsg

// Command-bus topics the UI and its controllers exchange. Keeping them
// here lets the compiler catch a typo and lets a reader jump from an
// emitter to its subscribers.
const (
	// Prompt intent, emitted by the UI.
	TopicInputText         = "user.input.text"
	TopicInputCommand      = "user.input.command"
	TopicInputSlashCommand = "user.input.slashcommand"
	TopicInputCancel       = "user.input.cancel"

	// Shortcuts and panel requests, emitted by the UI.
	TopicShortcutHelp   = "shortcut.help"
	TopicDebugView      = "debug.view"
	TopicDebugClear     = "debug.clear"
	TopicDebugCopy      = "debug.copy"
	TopicContextRefresh = "context.refresh"
	TopicPasteMultiline = "paste.multiline"

	// State changes, emitted by commands and controllers.
	TopicThemeChanged    = "theme.changed"
	TopicPersonaChanged  = "persona.changed"
	TopicVimModeChanged  = "vim.mode.changed"
	TopicCursorChanged   = "ui.cursor.changed"
	TopicMouseChanged    = "ui.mouse.changed"
	TopicRequestStarted  = "request.started"
	TopicRequestFinished = "request.finished"
	TopicTokenCount      = "token.count"
	TopicAppExit         = "app.exit"
)

// Core event-bus topics for confirmations.
const (
	CoreToolConfirmationRequest  = "tool.confirmation.request"
	CoreToolConfirmationResponse = "tool.confirmation.response"
	CoreUserConfirmationRequest  = "user.confirmation.request"
	CoreUserConfirmationResponse = "user.confirmation.response"
)
