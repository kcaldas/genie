package tui

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/pkg/genie"
)

type TUI struct {
	app *App
}

func New(genieService genie.Genie, session *genie.Session) (*TUI, error) {
	app, err := NewApp(genieService, session)
	if err != nil {
		return nil, err
	}
	
	return &TUI{app: app}, nil
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