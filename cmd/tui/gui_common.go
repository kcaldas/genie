package tui

import (
	"github.com/awesome-gocui/gocui"
)

type guiCommon struct {
	app *App
}

func (g *guiCommon) GetGui() *gocui.Gui {
	return g.app.gui
}

func (g *guiCommon) PostUIUpdate(fn func()) {
	g.app.gui.Update(func(*gocui.Gui) error {
		fn()
		return nil
	})
}
