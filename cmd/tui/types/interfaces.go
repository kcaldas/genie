package types

import (
	"time"

	"github.com/awesome-gocui/gocui"
)

type Component interface {
	GetKey() string
	GetView() *gocui.View
	GetViewName() string
	GetWindowName() string

	HandleFocus() error
	HandleFocusLost() error

	GetKeybindings() []*KeyBinding

	Render() error

	HasControlledBounds() bool
	IsTransient() bool

	// UI properties that define how this component should be displayed
	GetWindowProperties() WindowProperties
	GetTitle() string
}

type Controller interface {
	GetComponent() Component
	HandleCommand(command string) error
	HandleInput(input string) error
}

type State interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

type IGuiCommon interface {
	GetGui() *gocui.Gui
	GetConfig() *Config
	GetTheme() *Theme

	SetCurrentComponent(ctx Component)
	GetCurrentComponent() Component

	PostUIUpdate(func())
}

type IPopupHandler interface {
	ShowError(title, message string) error
	ShowConfirmation(title, message string, onConfirm func() error) error
	ShowPrompt(title, initialValue string, onSubmit func(string) error) error
	ClosePopup() error
}

type IComponentManager interface {
	Push(ctx Component) error
	Pop() error
	GetCurrent() Component
	GetAll() []Component
}

type IStateAccessor interface {
	GetMessages() []Message
	AddMessage(msg Message)
	ClearMessages()

	// Message range access for yank functionality
	GetMessageCount() int
	GetMessageRange(start, count int) []Message
	GetLastMessages(count int) []Message

	GetDebugMessages() []string
	AddDebugMessage(msg string)
	ClearDebugMessages()

	IsLoading() bool
	SetLoading(loading bool)
	GetLoadingDuration() time.Duration

	// Confirmation state
	SetWaitingConfirmation(waiting bool)
	IsWaitingConfirmation() bool
}

type ILayoutManager interface {
	SetWindowComponent(windowName string, component Component)
}

type IStatusComponent interface {
	SetLeftToReady()
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
}

