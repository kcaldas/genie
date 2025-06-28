package tui2

import (
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
	return t.app.Run()
}

func (t *TUI) Stop() {
	t.app.Close()
}