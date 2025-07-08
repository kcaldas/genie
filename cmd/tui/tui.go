package tui

import (
	"github.com/awesome-gocui/gocui"
)

type TUI struct {
	app *App
}

// New creates a TUI with an injected App instance
func New(app *App) *TUI {
	return &TUI{app: app}
}

func (t *TUI) Start() error {
	err := t.app.Run()
	// Handle gocui.ErrQuit as successful exit, not an error
	if err == gocui.ErrQuit {
		return nil
	}
	return err
}

func (t *TUI) Stop() {
	t.app.Close()
}

// GetApp returns the internal App instance for testing
func (t *TUI) GetApp() *App {
	return t.app
}