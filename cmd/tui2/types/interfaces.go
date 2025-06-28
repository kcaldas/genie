package types

import (
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
	
	GetDebugMessages() []string
	AddDebugMessage(msg string)
	
	IsLoading() bool
	SetLoading(loading bool)
}