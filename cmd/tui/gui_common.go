package tui

import (
	"github.com/awesome-gocui/gocui"
)

type Gui struct {
	gui *gocui.Gui
}

func (g *Gui) GetGui() *gocui.Gui {
	return g.gui
}

func (g *Gui) PostUIUpdate(fn func()) {
	g.gui.Update(func(*gocui.Gui) error {
		fn()
		return nil
	})
}
