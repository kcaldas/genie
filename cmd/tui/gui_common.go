package tui

import (
	"github.com/awesome-gocui/gocui"
)

type Gui struct {
	app *App
}

func (g *Gui) GetGui() *gocui.Gui {
	return g.app.gui
}

func (g *Gui) PostUIUpdate(fn func()) {
	g.app.gui.Update(func(*gocui.Gui) error {
		fn()
		return nil
	})
}
