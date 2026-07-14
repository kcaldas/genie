package types

import (
	"github.com/awesome-gocui/gocui"
)

type Component interface {
	GetKey() string
	GetView() *gocui.View
	GetViewName() string

	HandleFocus() error
	HandleFocusLost() error

	GetKeybindings() []*KeyBinding

	Render() error

	HasControlledBounds() bool

	// UI properties that define how this component should be displayed
	GetWindowProperties() WindowProperties
	GetTitle() string
}

type WindowProperties struct {
	// Basic properties
	Focusable  bool
	Editable   bool
	Wrap       bool
	Autoscroll bool
	Highlight  bool
	Frame      bool

	// Border configuration
	BorderStyle BorderStyle // What type of border
	BorderColor string      // Override border color (empty = use theme)
	FocusBorder bool        // Show special focus border

	// Focus behavior
	FocusStyle FocusStyle // How to show focus state

	// Title properties
	Subtitle string // Subtitle shown on right side of border
}

type Gui interface {
	GetGui() *gocui.Gui
	PostUIUpdate(func())
}

type IStateAccessor interface {
	GetMessages() []Message
	// AddMessage appends a message and returns its stable ID.
	AddMessage(msg Message) int64
	ClearMessages()
	GetMessageCount() int
	// UpdateMessageByID mutates the message with the given stable ID,
	// returning false if it has been evicted.
	UpdateMessageByID(id int64, update func(*Message)) bool
	GetLastMessage() *Message

	// Confirmation state
	SetWaitingConfirmation(waiting bool)
	IsWaitingConfirmation() bool

	// UI state management
	IsContextViewerActive() bool
	SetContextViewerActive(active bool)
}

// LLMContextDataProvider is the interface components use to interact with the LLMContextController
type LLMContextDataProvider interface {
	GetContextData() map[string]string
	HandleComponentEvent(eventName string, data interface{}) error
}
