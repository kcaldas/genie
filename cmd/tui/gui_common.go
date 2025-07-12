package tui

import (
	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type guiCommon struct {
	app *App
}

func (g *guiCommon) GetGui() *gocui.Gui {
	return g.app.gui
}

func (g *guiCommon) GetTheme() *types.Theme {
	config := g.app.config.GetConfig()
	return presentation.GetThemeForMode(config.Theme, config.OutputMode)
}


func (g *guiCommon) PostUIUpdate(fn func()) {
	g.app.gui.Update(func(*gocui.Gui) error {
		fn()
		return nil
	})
}
